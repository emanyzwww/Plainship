package builder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/plainship/internal/manifest"
)

// TestMinifyDir_HTML 压缩 HTML, 保留 pre 内容, 跳过非目标文件.
func TestMinifyDir_HTML(t *testing.T) {
	dir := t.TempDir()
	htmlContent := "<html>\n  <body>\n    <p>hello</p>\n<pre>  code\n  block\n</pre>\n  </body>\n</html>\n"
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(htmlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "robots.txt"), []byte("User-agent: *\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := manifest.New("b1", "s1", "h")
	m.Add(manifest.FileEntry{Output: "index.html", Hash: "old", Type: manifest.TypeIndex})
	m.Add(manifest.FileEntry{Output: "robots.txt", Hash: "old", Type: manifest.TypeSEO})
	if err := minifyDir(dir, m); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "<html>\n  <body>") {
		t.Errorf("HTML 未压缩: %s", s)
	}
	// minify 会省略可省闭合标签 (如 </p>), 内容必须保留.
	if !strings.Contains(s, "hello") {
		t.Errorf("内容缺失: %s", s)
	}
	// pre 内部空白必须保留 (代码高亮块不被破坏).
	if !strings.Contains(s, "  code\n  block") {
		t.Errorf("pre 内容被破坏: %s", s)
	}
	// 非目标文件原样保留.
	robots, _ := os.ReadFile(filepath.Join(dir, "robots.txt"))
	if string(robots) != "User-agent: *\n" {
		t.Errorf("robots.txt 不应被压缩: %q", robots)
	}
	// 清单哈希已按压缩后内容刷新.
	for _, f := range m.Files {
		if f.Hash == "old" {
			t.Errorf("哈希未刷新: %s", f.Output)
		}
	}
}

// TestMinifyDir_CSSJS 压缩 CSS 与 JS (注释与多余空白).
func TestMinifyDir_CSSJS(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.css"), []byte("/* c */\nbody {\n  color: red;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("// c\nvar x = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := manifest.New("b", "s", "h")
	m.Add(manifest.FileEntry{Output: "app.css", Hash: "old", Type: manifest.TypeAsset})
	m.Add(manifest.FileEntry{Output: "app.js", Hash: "old", Type: manifest.TypeAsset})
	if err := minifyDir(dir, m); err != nil {
		t.Fatal(err)
	}
	css, _ := os.ReadFile(filepath.Join(dir, "app.css"))
	if strings.Contains(string(css), "/* c */") {
		t.Errorf("CSS 注释未压缩: %s", css)
	}
	if !strings.Contains(string(css), "color:red") {
		t.Errorf("CSS 内容缺失: %s", css)
	}
	js, _ := os.ReadFile(filepath.Join(dir, "app.js"))
	if strings.Contains(string(js), "// c") {
		t.Errorf("JS 注释未压缩: %s", js)
	}
	if !strings.Contains(string(js), "var x=1") {
		t.Errorf("JS 内容缺失: %s", js)
	}
}

// TestBuild_Minify 生产构建产物 HTML 压缩 (单行).
func TestBuild_Minify(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/测试文档.md", sampleDoc)
	if _, err := Build(s, nil); err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	html, err := os.ReadFile(filepath.Join(s.BuildDir(), "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	// block 元素间空白必须压缩 (minify 保留 inline 间空白以保证渲染).
	if !strings.Contains(string(html), "</header><main") {
		t.Errorf("block 间空白未压缩: %s", html)
	}
	if !strings.Contains(string(html), "</ul></main>") {
		t.Errorf("block 间空白未压缩 (列表后): %s", html)
	}
	if !strings.Contains(string(html), "测试文档") {
		t.Errorf("压缩后内容缺失: %s", html)
	}
	// assets 也压缩.
	css, err := os.ReadFile(filepath.Join(s.BuildDir(), "assets", "app.css"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(css), "\n") {
		t.Errorf("生产构建 CSS 应压缩: %s", css)
	}
}

// TestBuildDev_NotMinified dev 构建保持可读 (不压缩).
func TestBuildDev_NotMinified(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/测试文档.md", sampleDoc)
	if _, err := BuildDev(s, nil); err != nil {
		t.Fatalf("dev 构建失败: %v", err)
	}
	html, err := os.ReadFile(filepath.Join(s.BuildDir(), "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), "\n") {
		t.Errorf("dev 构建不应压缩: %s", html)
	}
}
