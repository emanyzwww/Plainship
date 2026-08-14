package dev

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCleanURLPath_RejectsTraversal 验证反斜杠形式的穿越路径一律被拒绝,
// 修复 Windows 下 path.Clean 不识别反斜杠导致的任意文件读取.
// 注: net/http 已把 %5c 解码为反斜杠, 此处直接传入解码后的路径.
func TestCleanURLPath_RejectsTraversal(t *testing.T) {
	for _, p := range []string{
		"/..\\..\\Windows\\win.ini",
		"/a\\..\\..\\etc\\passwd",
		"/..\\secret.txt",
	} {
		if _, err := cleanURLPath(p); err == nil {
			t.Errorf("路径 %q 应被拒绝", p)
		}
	}
}

// TestCleanURLPath_NormalizesDotSegments 验证 "/../" 形式会被 path.Clean
// 折叠为根内相对路径 (安全: join 后仍在 build 目录内).
func TestCleanURLPath_NormalizesDotSegments(t *testing.T) {
	for _, p := range []string{"/../etc/passwd", "/a/../../x", "/.."} {
		got, err := cleanURLPath(p)
		if err != nil {
			t.Errorf("路径 %q 不应报错: %v", p, err)
			continue
		}
		if strings.HasPrefix(got, "..") {
			t.Errorf("路径 %q 折叠后仍含 ..: %q", p, got)
		}
	}
}

func TestCleanURLPath_OK(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/", ""},
		{"", ""},
		{"/guide/", "guide"},
		{"/guide/index.html", "guide/index.html"},
		{"/assets/app.css", "assets/app.css"},
	}
	for _, tt := range tests {
		got, err := cleanURLPath(tt.in)
		if err != nil {
			t.Errorf("cleanURLPath(%q) 错误: %v", tt.in, err)
			continue
		}
		// SafeRelPath 返回系统分隔符形式, 统一为 / 比较.
		if filepath.ToSlash(got) != tt.want {
			t.Errorf("cleanURLPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestHandleStatic_RejectsTraversal 通过 HTTP 层验证遍历请求被拒绝,
// 且无法读取 build 目录之外的文件.
func TestHandleStatic_RejectsTraversal(t *testing.T) {
	buildDir := t.TempDir()
	// build 目录内放一个正常文件.
	if err := os.MkdirAll(filepath.Join(buildDir, "ok"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "ok", "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	// build 目录外放一个"秘密"文件.
	secret := filepath.Join(filepath.Dir(buildDir), "secret.txt")
	if err := os.WriteFile(secret, []byte("top-secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(secret)

	s := NewServer(buildDir)
	// 正常访问.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ok/", nil)
	s.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Body.String() != "ok" {
		t.Errorf("正常访问失败: %d %s", w.Code, w.Body.String())
	}

	// 遍历请求: 必须被拒绝, 且响应中不得包含秘密文件内容.
	for _, path := range []string{
		"/..%5c..%5csecret.txt",
		"/..\\..\\secret.txt",
		"/../secret.txt",
	} {
		w = httptest.NewRecorder()
		r = httptest.NewRequest(http.MethodGet, path, nil)
		s.Routes().ServeHTTP(w, r)
		if w.Code == http.StatusOK {
			t.Errorf("遍历路径 %q 不应返回 200 (body=%s)", path, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "top-secret") {
			t.Errorf("遍历路径 %q 泄露了文件内容", path)
		}
	}
}

// TestHandleStatic_HtmlInjection 验证 HTML 页面注入热更新脚本.
func TestHandleStatic_HtmlInjection(t *testing.T) {
	buildDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(buildDir, "index.html"), []byte("<html><body>x</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewServer(buildDir)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	s.Routes().ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), "/__plainship/live.js") {
		t.Errorf("HTML 未注入热更新脚本: %s", w.Body.String())
	}
}
