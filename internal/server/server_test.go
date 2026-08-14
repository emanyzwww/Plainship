package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/Plainship/internal/sync"
)

// setupServer 创建带临时数据目录的测试服务器.
func setupServer(t *testing.T) *Server {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "data"), "")
}

// syncBuild 模拟一次客户端同步, 返回响应.
func syncBuild(t *testing.T, s *Server, siteID, buildID string, files map[string]string, deletes []string) *sync.Response {
	t.Helper()
	var payloads []sync.FilePayload
	for rel, content := range files {
		payloads = append(payloads, sync.FilePayload{
			Path:    rel,
			Content: base64.StdEncoding.EncodeToString([]byte(content)),
		})
	}
	req := sync.Request{
		ProtocolVersion: sync.ProtocolVersion,
		SiteID:          siteID,
		BuildID:         buildID,
		Files:           payloads,
		Deletes:         deletes,
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/sites/"+siteID+"/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("同步返回 %d: %s", w.Code, w.Body.String())
	}
	var resp sync.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("同步失败: %s", resp.Message)
	}
	return &resp
}

func TestSyncAndServe(t *testing.T) {
	s := setupServer(t)
	resp := syncBuild(t, s, "my-docs", "build-001", map[string]string{
		"index.html":       "<h1>首页</h1>",
		"guide/index.html": "<h1>指南</h1>",
		"assets/app.css":   "body{}",
	}, nil)
	if !resp.Active || resp.StoredFiles != 3 {
		t.Errorf("响应异常: %+v", resp)
	}

	// 静态访问.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "<h1>首页</h1>" {
		t.Errorf("首页: %d %s", w.Code, w.Body.String())
	}

	// 子路径.
	req = httptest.NewRequest(http.MethodGet, "/guide/", nil)
	w = httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("指南页: %d", w.Code)
	}

	// 404.
	req = httptest.NewRequest(http.MethodGet, "/missing/", nil)
	w = httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("404 页: %d", w.Code)
	}

	// CSS Content-Type.
	req = httptest.NewRequest(http.MethodGet, "/assets/app.css", nil)
	w = httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if !bytes.Contains(w.Body.Bytes(), []byte("body")) {
		t.Error("CSS 内容缺失")
	}
}

// TestStatic_SingleSiteFallback 验证服务器只有一个已发布站点 (非 my-docs) 时, 根路径自动服务该站点.
func TestStatic_SingleSiteFallback(t *testing.T) {
	s := setupServer(t)
	syncBuild(t, s, "my-blog", "build-001", map[string]string{
		"index.html": "<h1>自定义站点</h1>",
	}, nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("根路径应自动服务唯一站点, 状态码 = %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "自定义站点") {
		t.Errorf("响应内容不正确: %s", w.Body.String())
	}
}

// TestStatic_LatestSiteOnRoot 验证多站点场景下根路径服务最近激活的站点.
// 回归测试: 客户端改 siteId 重新发布后, 根路径不再永远显示旧 my-docs 站点.
func TestStatic_LatestSiteOnRoot(t *testing.T) {
	s := setupServer(t)
	// 先发布 my-docs (旧站点).
	syncBuild(t, s, "my-docs", "build-001", map[string]string{
		"index.html": "<h1>旧站点 my-docs</h1>",
	}, nil)
	// 再发布另一个站点 (最新).
	syncBuild(t, s, "my-site-x", "build-001", map[string]string{
		"index.html": "<h1>新站点 my-site-x</h1>",
	}, nil)

	// 根路径应服务最近激活的 my-site-x.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("根路径状态码 = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "新站点 my-site-x") {
		t.Errorf("根路径应服务最近激活的站点: %s", w.Body.String())
	}

	// 显式 ?site= 仍可访问旧站点.
	req = httptest.NewRequest(http.MethodGet, "/?site=my-docs", nil)
	w = httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "旧站点 my-docs") {
		t.Errorf("?site=my-docs 应服务旧站点: %d %s", w.Code, w.Body.String())
	}
}

// TestPublishedSites 验证已发布站点列表.
func TestPublishedSites(t *testing.T) {
	s := setupServer(t)
	if got := s.PublishedSites(); len(got) != 0 {
		t.Errorf("初始已发布站点应为空: %v", got)
	}
	syncBuild(t, s, "my-docs", "build-001", map[string]string{"index.html": "x"}, nil)
	sites := s.PublishedSites()
	if len(sites) != 1 || sites[0] != "my-docs" {
		t.Errorf("已发布站点 = %v, 期望 [my-docs]", sites)
	}
}

func TestSync_DeleteHandling(t *testing.T) {
	s := setupServer(t)
	syncBuild(t, s, "my-docs", "build-001", map[string]string{
		"index.html":     "<h1>首页</h1>",
		"old/index.html": "<h1>旧文章</h1>",
	}, nil)
	// 第二次同步删除旧文章.
	syncBuild(t, s, "my-docs", "build-002", map[string]string{
		"index.html": "<h1>首页</h1>",
	}, []string{"old/index.html"})

	req := httptest.NewRequest(http.MethodGet, "/old/", nil)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("已删除页面应 404, 实际 %d", w.Code)
	}
	// 检查 release-002 文件系统上也已删除.
	if _, err := os.Stat(filepath.Join(s.releaseDir("my-docs", "build-002"), "old", "index.html")); err == nil {
		t.Error("release 中仍存在已删除文件")
	}
}

// TestSync_IncrementalInheritance 验证增量同步: 新 release 继承上一版本,
// 只上传变化文件也能激活, 且删除生效.
func TestSync_IncrementalInheritance(t *testing.T) {
	s := setupServer(t)
	// 首次全量上传.
	syncBuild(t, s, "my-docs", "build-001", map[string]string{
		"index.html":     "<h1>首页</h1>",
		"a/index.html":   "<h1>A</h1>",
		"b/index.html":   "<h1>B</h1>",
		"assets/app.css": "body{}",
	}, nil)
	// 增量: 只上传变化的 a, 删除 b.
	syncBuild(t, s, "my-docs", "build-002", map[string]string{
		"a/index.html": "<h1>A 更新</h1>",
	}, []string{"b/index.html"})

	dir := s.releaseDir("my-docs", "build-002")
	// 新 release 应为完整快照: 继承未变化的文件.
	for _, want := range []string{"index.html", "a/index.html", "assets/app.css"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(want))); err != nil {
			t.Errorf("release-002 缺少继承文件 %s", want)
		}
	}
	// 删除的文件不应存在.
	if _, err := os.Stat(filepath.Join(dir, "b", "index.html")); err == nil {
		t.Error("release-002 仍存在已删除文件 b/index.html")
	}
	// 更新的文件内容正确.
	data, _ := os.ReadFile(filepath.Join(dir, "a", "index.html"))
	if string(data) != "<h1>A 更新</h1>" {
		t.Errorf("a/index.html 内容 = %q", data)
	}
	// 激活后静态访问: 更新生效, 删除生效.
	req := httptest.NewRequest(http.MethodGet, "/a/", nil)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Body.String() != "<h1>A 更新</h1>" {
		t.Errorf("访问 /a/ = %q", w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/b/", nil)
	w = httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("/b/ 应 404, 实际 %d", w.Code)
	}
}

// TestSync_NoIndexFails 验证缺少 index.html 时拒绝激活.
func TestSync_NoIndexFails(t *testing.T) {
	s := setupServer(t)
	req := sync.Request{
		ProtocolVersion: sync.ProtocolVersion,
		SiteID:          "my-docs",
		BuildID:         "build-001",
		Files: []sync.FilePayload{{
			Path:    "a/index.html",
			Content: base64.StdEncoding.EncodeToString([]byte("只有文章页")),
		}},
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/sites/my-docs/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("缺少 index.html 应 500, 实际 %d", w.Code)
	}
	var resp sync.Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.OK {
		t.Error("响应不应为 OK")
	}
}

func TestSync_AtomicActivation(t *testing.T) {
	s := setupServer(t)
	syncBuild(t, s, "my-docs", "build-001", map[string]string{
		"index.html": "<h1>第一版</h1>",
	}, nil)
	syncBuild(t, s, "my-docs", "build-002", map[string]string{
		"index.html": "<h1>第二版</h1>",
	}, nil)

	// 当前激活的应是最新版本.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, req)
	if w.Body.String() != "<h1>第二版</h1>" {
		t.Errorf("应激活第二版: %s", w.Body.String())
	}
	// 两个 release 都应保留.
	if _, err := os.Stat(s.releaseDir("my-docs", "build-001")); err != nil {
		t.Error("build-001 应保留")
	}
}

// TestSync_FullSyncClearsStale 验证 fullSync 请求不继承旧 release,
// 旧版本独有的文件不会残留.
func TestSync_FullSyncClearsStale(t *testing.T) {
	s := setupServer(t)
	syncBuild(t, s, "my-docs", "build-001", map[string]string{
		"index.html":     "<h1>首页</h1>",
		"old/x.html":     "旧文件",
		"assets/app.css": "body{}",
	}, nil)
	// 全量同步: 只上传新文件, 不带 deletes; 服务器必须整体重建.
	req := sync.Request{
		ProtocolVersion: sync.ProtocolVersion,
		SiteID:          "my-docs",
		BuildID:         "build-002",
		FullSync:        true,
		Files: []sync.FilePayload{{
			Path:    "index.html",
			Content: base64.StdEncoding.EncodeToString([]byte("<h1>第二版</h1>")),
		}},
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/sites/my-docs/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("fullSync 返回 %d: %s", w.Code, w.Body.String())
	}
	// 旧版本独有的文件不应残留.
	dir := s.releaseDir("my-docs", "build-002")
	if _, err := os.Stat(filepath.Join(dir, "old", "x.html")); err == nil {
		t.Error("fullSync 后旧文件仍残留")
	}
	// 激活后静态访问只有新内容.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	w2 := httptest.NewRecorder()
	s.Routes().ServeHTTP(w2, req2)
	if !strings.Contains(w2.Body.String(), "第二版") {
		t.Errorf("激活内容不正确: %s", w2.Body.String())
	}
}

// TestSync_SameBuildIDResync 验证同 buildID 重传不清空目录 (客户端重试场景),
// 增量文件与已有文件合并, 不会丢失.
func TestSync_SameBuildIDResync(t *testing.T) {
	s := setupServer(t)
	syncBuild(t, s, "my-docs", "build-001", map[string]string{
		"index.html":   "<h1>首页</h1>",
		"a/index.html": "<h1>A</h1>",
	}, nil)
	// 同 buildID 重推: 只带差异文件 a.
	syncBuild(t, s, "my-docs", "build-001", map[string]string{
		"a/index.html": "<h1>A 更新</h1>",
	}, nil)
	dir := s.releaseDir("my-docs", "build-001")
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		t.Error("同 buildID 重推后 index.html 不应丢失")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "a", "index.html"))
	if string(data) != "<h1>A 更新</h1>" {
		t.Errorf("a/index.html = %q", data)
	}
}

// TestReleaseInfo_RequiresAuth 验证 release 元数据接口需要鉴权.
func TestReleaseInfo_RequiresAuth(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "data"), "secret-token")
	// 无令牌被拒.
	r := httptest.NewRequest(http.MethodGet, "/api/v1/sites/my-docs/releases/build-001", nil)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("无令牌访问 release info 应 401, 实际 %d", w.Code)
	}
	// 有令牌可通过.
	r = httptest.NewRequest(http.MethodGet, "/api/v1/sites/my-docs/releases/build-001", nil)
	r.Header.Set("Authorization", "Bearer secret-token")
	w = httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if w.Code == http.StatusUnauthorized {
		t.Error("带正确令牌不应 401")
	}
}

// TestServe_RefusesEmptyToken 验证空令牌时 Serve 拒绝启动 (认证永远开启).
func TestServe_RefusesEmptyToken(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "data"), "")
	if err := s.Serve("127.0.0.1:0"); err == nil {
		t.Error("空令牌的服务器不应启动")
	}
}

func TestSync_ProtocolVersionMismatch(t *testing.T) {
	s := setupServer(t)
	req := sync.Request{
		ProtocolVersion: 999,
		SiteID:          "my-docs",
		BuildID:         "build-001",
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/sites/my-docs/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("协议版本不匹配应 400, 实际 %d", w.Code)
	}
}

func TestSync_PathTraversalRejected(t *testing.T) {
	s := setupServer(t)
	req := sync.Request{
		ProtocolVersion: sync.ProtocolVersion,
		SiteID:          "my-docs",
		BuildID:         "build-001",
		Files: []sync.FilePayload{{
			Path:    "../../../etc/passwd",
			Content: base64.StdEncoding.EncodeToString([]byte("evil")),
		}},
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/sites/my-docs/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("路径遍历应 400, 实际 %d", w.Code)
	}
	// 确保没有文件被写入 release 目录之外.
	if _, err := os.Stat(filepath.Join(s.DataDir, "..", "..", "etc", "passwd")); err == nil {
		t.Error("路径遍历文件被写入!")
	}
}

func TestAuth_TokenRequired(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "data"), "secret-token")
	// 无 Token 同步被拒.
	req := sync.Request{
		ProtocolVersion: sync.ProtocolVersion,
		SiteID:          "my-docs",
		BuildID:         "build-001",
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/sites/my-docs/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("无 Token 应 401, 实际 %d", w.Code)
	}
	// 错误 Token 被拒.
	r = httptest.NewRequest(http.MethodPost, "/api/v1/sites/my-docs/sync", bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer wrong")
	w = httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("错误 Token 应 401, 实际 %d", w.Code)
	}
	// 正确 Token 通过.
	r = httptest.NewRequest(http.MethodPost, "/api/v1/sites/my-docs/sync", bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer secret-token")
	w = httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if w.Code == http.StatusUnauthorized {
		t.Error("正确 Token 不应 401")
	}
}

func TestStatusEndpoint(t *testing.T) {
	s := setupServer(t)
	// 未发布.
	r := httptest.NewRequest(http.MethodGet, "/api/v1/sites/my-docs/status", nil)
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	// 发布后.
	syncBuild(t, s, "my-docs", "build-001", map[string]string{"index.html": "<h1>x</h1>"}, nil)
	r = httptest.NewRequest(http.MethodGet, "/api/v1/sites/my-docs/status", nil)
	w = httptest.NewRecorder()
	s.Routes().ServeHTTP(w, r)
	if !bytes.Contains(w.Body.Bytes(), []byte("build-001")) {
		t.Errorf("status 应包含激活构建: %s", w.Body.String())
	}
}
