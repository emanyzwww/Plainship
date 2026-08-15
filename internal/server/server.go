// Package server 实现 Plainship Server.
//
// 服务器只做三件事: 存储, 同步, 静态 HTTP, 不做任何 Markdown 编译.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/emanyzwww/plainship/internal/i18n"
)

// 文件名与站点 ID 的安全模式.
var (
	siteIDPattern  = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$")
	buildIDPattern = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$")
)

// MaxFileSize 是单个上传文件的字节上限, 64MB.
const MaxFileSize = 64 << 20

// MaxBodySize 是同步请求体上限, 1GB.
const MaxBodySize = 1 << 30

// MaxDecodedSize 是单次同步请求中所有文件解码后的总字节上限, 512MB.
// 限制 base64 解码后的内存占用, 防止恶意请求拖垮服务器.
const MaxDecodedSize = 512 << 20

// Server 是 Plainship 服务器.
type Server struct {
	DataDir string       // DataDir 是数据目录, 例如 data/.
	Token   string       // Token 是同步接口的 Bearer Token, 为空表示不校验.
	Log     *slog.Logger // Log 是运行日志, 同步/激活/鉴权事件, nil 表示不记日志.
}

// New 创建服务器实例.
func New(dataDir, token string) *Server {
	return &Server{DataDir: dataDir, Token: token}
}

// WithLogger 绑定运行日志, 用于同步/激活/鉴权事件记录.
func (s *Server) WithLogger(l *slog.Logger) *Server {
	s.Log = l
	return s
}

// log 记录一条服务器事件; 未绑定日志时静默.
func (s *Server) log(level slog.Level, msg string, args ...any) {
	if s.Log == nil {
		return
	}
	s.Log.Log(context.Background(), level, msg, args...)
}

// Routes 返回 HTTP 路由.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/sites/{siteID}/sync", s.handleSync)
	mux.HandleFunc("GET /api/v1/sites/{siteID}/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/sites/{siteID}/releases/{buildID}", s.handleReleaseInfo)
	mux.HandleFunc("/", s.handleStatic)
	return mux
}

// Serve 启动 HTTP 服务.
//
// 认证永远开启: 空令牌时拒绝启动, 防止服务器以无认证状态运行.
func (s *Server) Serve(addr string) error {
	return s.ServeHandler(addr, s.Routes())
}

// ServeHandler 与 Serve 相同, 但允许调用方包装路由处理器, 如访问日志中间件.
func (s *Server) ServeHandler(addr string, h http.Handler) error {
	if s.Token == "" {
		return i18n.Errorf(i18n.ServerAuthNoToken)
	}
	return http.ListenAndServe(addr, h)
}

// ---- 存储布局 ----
// `data/sites/<siteID>/releases/<buildID>/...`  每次构建的完整产物.
// `data/sites/<siteID>/current.json`            当前激活的 buildID 指针.
// `data/sites/<siteID>/releases/<buildID>/release.json`  构建元数据.

// siteDir 返回站点数据目录.
func (s *Server) siteDir(siteID string) string {
	return filepath.Join(s.DataDir, "sites", siteID)
}

// releaseDir 返回一次构建的产物目录.
func (s *Server) releaseDir(siteID, buildID string) string {
	return filepath.Join(s.siteDir(siteID), "releases", buildID)
}

// currentPtr 是 current.json 的结构.
type currentPtr struct {
	BuildID     string `json:"buildId"`
	ActivatedAt string `json:"activatedAt"`
}

// currentFilePath 返回当前激活指针文件路径.
func (s *Server) currentFilePath(siteID string) string {
	return filepath.Join(s.siteDir(siteID), "current.json")
}

// activeBuildID 读取当前激活的 buildID.
func (s *Server) activeBuildID(siteID string) (string, error) {
	data, err := os.ReadFile(s.currentFilePath(siteID))
	if err != nil {
		return "", err
	}
	var ptr currentPtr
	if err := json.Unmarshal(data, &ptr); err != nil {
		return "", err
	}
	if ptr.BuildID == "" {
		return "", os.ErrNotExist
	}
	return ptr.BuildID, nil
}

// PublishedSites 返回 data/sites 下所有已发布站点 ID.
func (s *Server) PublishedSites() []string {
	entries, err := os.ReadDir(filepath.Join(s.DataDir, "sites"))
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := s.activeBuildID(e.Name()); err == nil {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	return ids
}

// latestPublishedSite 返回最近一次激活的站点 ID.
//
// 多站点场景下, 根路径, 无 `?site=` 参数, 服务最近激活的站点.
func (s *Server) latestPublishedSite() string {
	ids := s.PublishedSites()
	if len(ids) == 0 {
		return ""
	}
	if len(ids) == 1 {
		return ids[0]
	}
	latest := ""
	latestAt := time.Time{}
	for _, id := range ids {
		data, err := os.ReadFile(s.currentFilePath(id))
		if err != nil {
			continue
		}
		var ptr currentPtr
		if err := json.Unmarshal(data, &ptr); err != nil {
			continue
		}
		// RFC3339 布局可同时解析 RFC3339 与 RFC3339Nano 两种格式.
		t, err := time.Parse(time.RFC3339, ptr.ActivatedAt)
		if err != nil {
			continue
		}
		if t.After(latestAt) {
			latestAt = t
			latest = id
		}
	}
	if latest == "" {
		// 无有效激活时间时回退到排序后的第一个站点.
		return ids[0]
	}
	return latest
}

// writeJSON 输出 JSON 响应.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
