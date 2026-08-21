package output

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/papership-client/core/derive"
	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/emanyzwww/papership-client/core/render"
	"github.com/emanyzwww/papership-client/core/scanner/scanner"
	"github.com/emanyzwww/papership-client/internal/testutil"
	"github.com/emanyzwww/papership-client/model/space"
)

// fullInput 构造包含页面/资源/主题 static/派生数据的完整输入.
func fullInput(t *testing.T) (*space.Space, *Input) {
	t.Helper()
	sp := testutil.NewSpace(t, map[string]string{
		"docs/index.md":                   "# 首页\n",
		"docs/guide/intro.md":             "# 入门\n",
		"docs/img/logo.png":               "pngdata",
		"favicon.ico":                     "icodata",
		"themes/fancy/static/css/app.css": "body{}",
	})
	sp.Config.SiteURL = "https://example.com"
	in := &Input{
		Space: sp,
		Theme: "fancy",
		Pages: []render.Page{
			{Page: derive.Page{}, HTML: []byte("<h1>首页</h1>"), OutPath: "index.html"},
			{Page: derive.Page{}, HTML: []byte("<h1>入门</h1>"), OutPath: "guide/intro/index.html"},
		},
		Assets: []scanner.AssetEntry{
			{AbsPath: filepath.Join(sp.Root, "docs", "img", "logo.png"), RelPath: "docs/img/logo.png"},
			{AbsPath: filepath.Join(sp.Root, "favicon.ico"), RelPath: "favicon.ico"},
		},
		SiteMap: []string{"/", "/guide/intro/"},
		Search: []derive.SearchEntry{
			{URL: "/", Title: "首页", Text: "欢迎"},
			{URL: "/guide/intro/", Title: "入门", Text: "这是正文"},
		},
	}
	return sp, in
}

// mustRead 断言文件存在并返回内容.
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	return data
}

// TestWriteFullSite 锁定整站出盘: 页面/资源/主题 static/附加文件全部落盘且内容正确.
func TestWriteFullSite(t *testing.T) {
	sp, in := fullInput(t)
	res, err := Write(context.Background(), in)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(res.Problems) != 0 {
		t.Errorf("Problems = %+v, want 0", res.Problems)
	}

	build := filepath.Join(sp.Root, "build")
	// 页面.
	if got := string(mustRead(t, filepath.Join(build, "index.html"))); got != "<h1>首页</h1>" {
		t.Errorf("index.html = %q", got)
	}
	if got := string(mustRead(t, filepath.Join(build, "guide", "intro", "index.html"))); got != "<h1>入门</h1>" {
		t.Errorf("guide/intro/index.html = %q", got)
	}
	// 静态资源: docs 前缀被剥离, 根级资源原样.
	if got := string(mustRead(t, filepath.Join(build, "img", "logo.png"))); got != "pngdata" {
		t.Errorf("img/logo.png = %q", got)
	}
	if got := string(mustRead(t, filepath.Join(build, "favicon.ico"))); got != "icodata" {
		t.Errorf("favicon.ico = %q", got)
	}
	// 主题 static.
	if got := string(mustRead(t, filepath.Join(build, "css", "app.css"))); got != "body{}" {
		t.Errorf("css/app.css = %q", got)
	}
	// 附加文件.
	sitemap := string(mustRead(t, filepath.Join(build, "sitemap.xml")))
	if !strings.Contains(sitemap, "https://example.com/</loc>") ||
		!strings.Contains(sitemap, "https://example.com/guide/intro/</loc>") {
		t.Errorf("sitemap.xml 缺 loc: %q", sitemap)
	}
	var idx []derive.SearchEntry
	if err := json.Unmarshal(mustRead(t, filepath.Join(build, "search-index.json")), &idx); err != nil {
		t.Fatalf("search-index.json 解析失败: %v", err)
	}
	if len(idx) != 2 || idx[1].URL != "/guide/intro/" || idx[1].Title != "入门" {
		t.Errorf("search-index.json = %+v", idx)
	}
	robots := string(mustRead(t, filepath.Join(build, "robots.txt")))
	if !strings.Contains(robots, "Sitemap: https://example.com/sitemap.xml") {
		t.Errorf("robots.txt = %q, 应含 Sitemap 行", robots)
	}

	// 文件清单: 2 页 + 2 资源 + 1 主题 + 3 附加 = 8.
	if res.DocCount() != 8 {
		t.Errorf("DocCount = %d, want 8 (写盘清单)", res.DocCount())
	}
	foundPage := false
	for _, w := range res.Docs {
		if w.Path == "guide/intro/index.html" && w.Bytes == int64(len("<h1>入门</h1>")) {
			foundPage = true
		}
	}
	if !foundPage {
		t.Errorf("写盘清单缺页面条目: %+v", res.Docs)
	}
}

// TestWriteCustomLayout 锁定自定义文档根: content/... 的资源前缀剥离.
func TestWriteCustomLayout(t *testing.T) {
	sp := testutil.NewSpaceWithLayout(t, space.Layout{Docs: "content", Themes: "skins", Build: "build"}, map[string]string{
		"content/img/x.png": "xdata",
	})
	in := &Input{
		Space: sp,
		Assets: []scanner.AssetEntry{
			{AbsPath: filepath.Join(sp.Root, "content", "img", "x.png"), RelPath: "content/img/x.png"},
		},
	}
	res, err := Write(context.Background(), in)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := string(mustRead(t, filepath.Join(sp.Root, "build", "img", "x.png"))); got != "xdata" {
		t.Errorf("build/img/x.png = %q, want xdata (剥离 content/)", got)
	}
	_ = res
}

// TestWriteAssetMissing 锁定资源缺失容错: warning 问题, 其余文件照常写盘.
func TestWriteAssetMissing(t *testing.T) {
	sp, in := fullInput(t)
	in.Assets = append(in.Assets, scanner.AssetEntry{
		AbsPath: filepath.Join(sp.Root, "docs", "gone.png"),
		RelPath: "docs/gone.png",
	})
	res, err := Write(context.Background(), in)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	hasWarning := false
	for _, p := range res.Problems {
		if p.Stage == "output" && p.Severity == pipeline.SeverityWarning && p.Path == "docs/gone.png" {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Errorf("资源缺失未收集 warning: %+v", res.Problems)
	}
	if _, err := os.Stat(filepath.Join(sp.Root, "build", "index.html")); err != nil {
		t.Error("index.html 应正常写出")
	}
}

// TestWriteNilInput 锁定 nil 输入报错.
func TestWriteNilInput(t *testing.T) {
	if _, err := Write(context.Background(), nil); err == nil {
		t.Fatal("Write(nil): expected error, got nil")
	}
}

// TestWriteStage 锁定 Stage 冒烟.
func TestWriteStage(t *testing.T) {
	sp, in := fullInput(t)
	res, err := (Stage{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Stage.Run: %v", err)
	}
	if res.DocCount() == 0 {
		t.Error("无文件写出")
	}
	_ = sp
}
