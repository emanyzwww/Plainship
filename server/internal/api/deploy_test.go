package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/emanyzwww/papership-shared/proto"
)

// buildBundle 构造 gzip+tar 包体, 条目为给定文件.
func buildBundle(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// deployMux 构造带部署端点与静态服务的完整 mux.
func deployMux(t *testing.T, sitesDir, token string) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(proto.DeployPath, DeployHandler(sitesDir, token))
	mux.Handle("/", http.FileServer(http.Dir(sitesDir)))
	return mux
}

// postDeploy 组装 multipart 请求并交给 handler.
func postDeploy(t *testing.T, h http.Handler, siteID string, bundle []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	meta, _ := json.Marshal(proto.DeployRequest{SiteID: siteID, PayloadType: "tar.gz", Files: 1, Size: int64(len(bundle))})
	if err := w.WriteField("meta", string(meta)); err != nil {
		t.Fatal(err)
	}
	fw, err := w.CreateFormFile("bundle", siteID+".tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(bundle); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, proto.DeployPath, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// TestDeployAndServe 锁定完整链路: 部署后站点目录可被静态服务访问.
func TestDeployAndServe(t *testing.T) {
	sitesDir := t.TempDir()
	mux := deployMux(t, sitesDir, "")
	bundle := buildBundle(t, map[string]string{"index.html": "<h1>hi</h1>", "css/app.css": "body{}"})

	rr := postDeploy(t, mux, "demo", bundle, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("deploy code = %d, body=%s", rr.Code, rr.Body.String())
	}
	var dr proto.DeployResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &dr); err != nil {
		t.Fatal(err)
	}
	if !dr.OK || dr.Revision == "" || dr.SiteURL != "/demo/" {
		t.Errorf("response = %+v", dr)
	}

	live := filepath.Join(sitesDir, "demo")
	if _, err := os.Stat(filepath.Join(live, "index.html")); err != nil {
		t.Fatalf("站点目录未落地: %v", err)
	}
	// clean URL 约定访问目录 URL; FileServer 会把 /demo/index.html 301 到 ./ (浏览器再走目录),
	// 直接断言目录 URL 出 index.html 内容.
	get := httptest.NewRequest(http.MethodGet, "/demo/", nil)
	grr := httptest.NewRecorder()
	mux.ServeHTTP(grr, get)
	if grr.Code != http.StatusOK || grr.Body.String() != "<h1>hi</h1>" {
		t.Errorf("静态服务 = %d %q", grr.Code, grr.Body.String())
	}
}

// TestDeployBadSiteID 锁定站点标识校验: 路径分隔符与 .. 拒绝.
func TestDeployBadSiteID(t *testing.T) {
	sitesDir := t.TempDir()
	rr := postDeploy(t, DeployHandler(sitesDir, ""), "../evil", buildBundle(t, map[string]string{"x": "1"}), "")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", rr.Code)
	}
	if _, err := os.Stat(filepath.Join(sitesDir, "evil")); err == nil {
		t.Error("越界站点目录被创建")
	}
}

// TestDeployTraversal 锁定包内路径越界防护: ../ 条目拒绝且不落盘.
func TestDeployTraversal(t *testing.T) {
	sitesDir := t.TempDir()
	bundle := buildBundle(t, map[string]string{"../evil.txt": "bad"})
	rr := postDeploy(t, DeployHandler(sitesDir, ""), "demo", bundle, "")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", rr.Code)
	}
	if _, err := os.Stat(filepath.Join(sitesDir, "evil.txt")); err == nil {
		t.Error("越界文件被写出")
	}
}

// TestDeployToken 锁定鉴权: 配置令牌后无令牌请求被拒.
func TestDeployToken(t *testing.T) {
	sitesDir := t.TempDir()
	h := DeployHandler(sitesDir, "secret")
	bundle := buildBundle(t, map[string]string{"index.html": "x"})

	if rr := postDeploy(t, h, "demo", bundle, ""); rr.Code != http.StatusUnauthorized {
		t.Errorf("无令牌 code = %d, want 401", rr.Code)
	}
	if rr := postDeploy(t, h, "demo", bundle, "secret"); rr.Code != http.StatusOK {
		t.Errorf("带令牌 code = %d, want 200", rr.Code)
	}
}
