package distribution

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/papership-client/model/space"
	"github.com/emanyzwww/papership-shared/proto"
)

// mkBuild 构造带构建产物的临时 Space (BuildDir 含两个文件).
func mkBuild(t *testing.T) *space.Space {
	t.Helper()
	sp := &space.Space{Root: t.TempDir(), Layout: space.DefaultLayout()}
	if err := os.MkdirAll(filepath.Join(sp.Root, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"index.html", "docs/guide/index.html"} {
		abs := filepath.Join(sp.Root, "build", filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte("<html>"+f+"</html>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return sp
}

// tarNames 解包 gzip+tar 并返回条目名.
func tarNames(t *testing.T, data []byte) []string {
	t.Helper()
	gz, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
	}
	return names
}

// TestDistribute 锁定完整分发: 打包内容正确, 鉴权头携带, 响应解析成功.
func TestDistribute(t *testing.T) {
	var gotAuth string
	var gotMeta proto.DeployRequest
	var gotNames []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != proto.DeployPath || r.Method != http.MethodPost {
			http.Error(w, "wrong route", http.StatusNotFound)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal([]byte(r.FormValue("meta")), &gotMeta); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f, _, err := r.FormFile("bundle")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		b, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		gotNames = tarNames(t, b)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(proto.DeployResponse{OK: true, Revision: "r-42", SiteURL: "/demo/"})
	}))
	defer srv.Close()

	sp := mkBuild(t)
	sp.Config.ServerURL = srv.URL
	sp.Config.SiteID = "demo"
	sp.LocalConfig.ServerToken = "tok"
	res, err := Distribute(context.Background(), sp, Options{})
	if err != nil {
		t.Fatalf("Distribute: %v", err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("Authorization = %q, want Bearer tok", gotAuth)
	}
	if gotMeta.SiteID != "demo" || gotMeta.PayloadType != "tar.gz" || gotMeta.Files != 2 {
		t.Errorf("meta = %+v", gotMeta)
	}
	want := []string{"docs/guide/index.html", "index.html"}
	if len(gotNames) != 2 || gotNames[0] != want[0] || gotNames[1] != want[1] {
		t.Errorf("包体条目 = %v, want %v (排序)", gotNames, want)
	}
	if res.Revision != "r-42" || res.SiteURL != "/demo/" || res.Files != 2 || res.Bytes == 0 {
		t.Errorf("Result = %+v", res)
	}
}

// TestDistributeServerError 锁定服务端拒绝: 45x 时返回 error.
func TestDistributeServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(proto.DeployResponse{OK: false, Message: "磁盘满"})
	}))
	defer srv.Close()

	sp := mkBuild(t)
	sp.Config.ServerURL = srv.URL
	if _, err := Distribute(context.Background(), sp, Options{}); err == nil {
		t.Fatal("期望错误, 得到 nil")
	}
}

// TestDistributeEmptyBuild 锁定空产物拒绝分发.
func TestDistributeEmptyBuild(t *testing.T) {
	sp := &space.Space{Root: t.TempDir(), Layout: space.DefaultLayout()}
	sp.Config.ServerURL = "http://example.com"
	if _, err := Distribute(context.Background(), sp, Options{}); err == nil {
		t.Fatal("空构建产物应拒绝分发")
	}
}

// TestDistributeNoServerURL 锁定未配置服务端地址报错.
func TestDistributeNoServerURL(t *testing.T) {
	sp := mkBuild(t)
	if _, err := Distribute(context.Background(), sp, Options{}); err == nil {
		t.Fatal("未配置服务端地址应报错")
	}
}

// TestDistributeNilSpace 锁定 nil Space 报错.
func TestDistributeNilSpace(t *testing.T) {
	if _, err := Distribute(context.Background(), nil, Options{}); err == nil {
		t.Fatal("nil space 应报错")
	}
}

// TestDistributeStage 锁定 Stage 冒烟 (含 nil input).
func TestDistributeStage(t *testing.T) {
	if _, err := (Stage{}).Run(context.Background(), nil); err == nil {
		t.Fatal("Stage.Run(nil): expected error, got nil")
	}
}
