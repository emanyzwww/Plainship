package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/emanyzwww/plainship/internal/fsutil"
	"github.com/emanyzwww/plainship/internal/i18n"
)

// handleStatic 提供激活构建的静态文件.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// 仅支持 GET / HEAD.
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, i18n.T(i18n.ServerStaticMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	// 找出请求对应的站点.
	// 未指定 ?site= 时, 根路径服务最近激活的已发布站点 (单站点场景即该站点);
	// 没有任何已发布站点时回退默认 my-docs (返回"尚未发布"提示).
	// 未来可通过 Host 或 config 映射多站点.
	siteID := r.URL.Query().Get("site")
	if siteID == "" {
		siteID = s.latestPublishedSite()
		if siteID == "" {
			siteID = "my-docs"
		}
	}
	if !siteIDPattern.MatchString(siteID) {
		http.Error(w, i18n.T(i18n.ServerStaticSiteNotFound), http.StatusNotFound)
		return
	}
	active, err := s.activeBuildID(siteID)
	if err != nil {
		http.Error(w, i18n.T(i18n.ServerStaticNotPublished), http.StatusNotFound)
		return
	}
	rel, err := cleanURLPath(r.URL.Path)
	if err != nil {
		http.Error(w, i18n.T(i18n.ServerStaticPathInvalid), http.StatusBadRequest)
		return
	}
	base := s.releaseDir(siteID, active)
	// 目录请求 -> index.html.
	if strings.HasSuffix(r.URL.Path, "/") {
		rel = path.Join(rel, "index.html")
	}
	full := filepath.Join(base, filepath.FromSlash(rel))
	// 防遍历: 再次校验 (含路径边界分隔符, 防止 base 前缀误匹配兄弟目录).
	if full != base && !strings.HasPrefix(full, base+string(filepath.Separator)) {
		http.Error(w, i18n.T(i18n.ServerStaticPathInvalid), http.StatusForbidden)
		return
	}
	info, err := os.Lstat(full)
	if err != nil {
		// 无扩展名路径尝试补充 index.html, 并 301 重定向到目录形式.
		if !strings.HasSuffix(r.URL.Path, "/") && !hasExtension(r.URL.Path) {
			target := r.URL.Path + "/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		s.write404(w, siteID)
		return
	}
	if info.IsDir() {
		full = filepath.Join(full, "index.html")
		if _, err := os.Lstat(full); err != nil {
			s.write404(w, siteID)
			return
		}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		http.Error(w, i18n.T(i18n.ServerStaticSymlink), http.StatusForbidden)
		return
	}
	// 计算 ETag.
	data, err := os.ReadFile(full)
	if err != nil {
		s.write404(w, siteID)
		return
	}
	sum := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeContent(w, r, filepath.Base(full), info.ModTime(), bytes.NewReader(data))
}

// write404 输出 404 页面.
func (s *Server) write404(w http.ResponseWriter, siteID string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "<!DOCTYPE html><html><head><meta charset=\"UTF-8\"><title>%s</title></head><body><h1>404</h1><p>%s</p><p><a href=\"/\">%s</a></p></body></html>", i18n.T(i18n.ServerStaticNotFoundTitle), i18n.T(i18n.ServerStaticNotFound), i18n.T(i18n.ServerStaticBackHome))
}

// cleanURLPath 清理并校验 URL 路径, 返回相对路径.
// 使用 fsutil.SafeRelPath: 反斜杠先归一化为正斜杠, 任何 ".." 段都被拒绝,
// 避免 Windows 下反斜杠绕过 path.Clean 的目录穿越.
func cleanURLPath(urlPath string) (string, error) {
	decoded, err := url.PathUnescape(urlPath)
	if err != nil {
		return "", err
	}
	clean := path.Clean("/" + decoded)
	if clean == "/" {
		return "", nil
	}
	rel, err := fsutil.SafeRelPath(clean)
	if err != nil {
		return "", i18n.Errorf(i18n.ServerStaticPathTraversal)
	}
	return rel, nil
}

// hasExtension 判断路径最后一段是否包含扩展名.
func hasExtension(p string) bool {
	last := path.Base(p)
	return strings.Contains(last, ".")
}
