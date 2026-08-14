package dev

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/emanyzwww/Plainship/internal/fsutil"
)

// Server 是开发模式 HTTP 服务器.
// 直接服务 build/ 目录, 并在页面中注入热更新脚本.
// 通过 SSE 向所有已连接页面广播 reload 事件.
type Server struct {
	// BuildDir 是构建产物目录 (build/).
	BuildDir string

	mu      sync.Mutex
	clients map[chan string]struct{}
}

// NewServer 创建开发服务器.
func NewServer(buildDir string) *Server {
	return &Server{BuildDir: buildDir, clients: map[chan string]struct{}{}}
}

// Routes 返回 HTTP 路由.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/__plainship/events", s.handleEvents)
	mux.HandleFunc("/__plainship/live.js", s.handleLiveJS)
	mux.HandleFunc("/", s.handleStatic)
	return mux
}

// Broadcast 向所有已连接客户端广播事件.
// event 例如 "reload".
func (s *Server) Broadcast(event string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- event:
		default:
		}
	}
}

// handleEvents 是 SSE 端点: 浏览器通过 EventSource 订阅.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE 不支持", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 16)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	}()

	// 连接建立后立即发送一次 ready, 供客户端确认通道可用.
	fmt.Fprintf(w, "event: ready\ndata: connected\n\n")
	fl.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev, ev)
			fl.Flush()
		}
	}
}

// handleLiveJS 提供注入用的热更新客户端脚本.
func (s *Server) handleLiveJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	fmt.Fprint(w, liveJS)
}

// handleStatic 服务 build/ 目录, 并在 HTML 中注入热更新脚本.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// 仅支持 GET / HEAD.
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}
	rel, err := cleanURLPath(r.URL.Path)
	if err != nil {
		http.Error(w, "路径无效", http.StatusBadRequest)
		return
	}
	full := filepath.Join(s.BuildDir, filepath.FromSlash(rel))
	// 防遍历: join 后再次校验 (含路径边界分隔符).
	// 修复 Windows 下反斜杠绕过 path.Clean 导致的任意文件读取.
	if full != s.BuildDir && !strings.HasPrefix(full, s.BuildDir+string(filepath.Separator)) {
		http.Error(w, "路径无效", http.StatusForbidden)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/") {
		full = filepath.Join(full, "index.html")
	}
	info, err := os.Stat(full)
	if err != nil {
		// 无扩展名路径尝试补充 index.html.
		if !strings.HasSuffix(r.URL.Path, "/") && !hasExtension(r.URL.Path) {
			target := r.URL.Path + "/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		http.NotFound(w, r)
		return
	}
	if info.IsDir() {
		full = filepath.Join(full, "index.html")
		if _, err := os.Stat(full); err != nil {
			http.NotFound(w, r)
			return
		}
	}
	data, err := os.ReadFile(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// HTML 注入热更新脚本.
	if strings.HasSuffix(strings.ToLower(full), ".html") {
		injected := strings.Replace(string(data), "</body>", `<script src="/__plainship/live.js"></script></body>`, 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, injected)
		return
	}
	ct := mimeType(full)
	if ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	fmt.Fprint(w, string(data))
}

// cleanURLPath 清理并校验 URL 路径, 返回相对路径.
// 复用 fsutil.SafeRelPath: 反斜杠先归一化为正斜杠, 任何 ".." 段都被拒绝,
// 修复 Windows 下 path.Clean 不识别反斜杠导致的目录穿越.
func cleanURLPath(urlPath string) (string, error) {
	if urlPath == "" || urlPath == "/" {
		return "", nil
	}
	clean := path.Clean("/" + urlPath)
	if clean == "/" {
		return "", nil
	}
	rel, err := fsutil.SafeRelPath(clean)
	if err != nil {
		return "", fmt.Errorf("路径包含非法部分")
	}
	return rel, nil
}

// hasExtension 判断路径最后一段是否包含扩展名.
func hasExtension(p string) bool {
	last := path.Base(p)
	return strings.Contains(last, ".")
}

// mimeType 按扩展名返回 Content-Type.
func mimeType(p string) string {
	lower := strings.ToLower(filepath.Ext(p))
	switch lower {
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".woff", ".woff2":
		return "font/woff2"
	case ".xml":
		return "application/xml; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// liveJS 是注入到页面的热更新客户端.
// 通过 EventSource 订阅 reload 事件, 收到后刷新页面.
const liveJS = `
// Plainship 热更新客户端.
// 通过 EventSource 订阅 /__plainship/events,
// 收到 reload 事件后刷新页面.
(function () {
    function connect() {
        var es = new EventSource("/__plainship/events");
        es.addEventListener("ready", function () {
            console.log("[Plainship] 热更新已连接");
        });
        es.addEventListener("reload", function () {
            console.log("[Plainship] 检测到更新, 刷新页面...");
            es.close();
            window.location.reload();
        });
        es.onerror = function () {
            // 服务器重启时自动重连.
            es.close();
            setTimeout(connect, 1000);
        };
    }
    connect();
})();
`
