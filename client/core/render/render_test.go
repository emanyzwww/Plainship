package render

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/emanyzwww/papership-client/core/assembly/document"
	"github.com/emanyzwww/papership-client/core/derive"
	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/emanyzwww/papership-client/internal/testdoc"
	"github.com/emanyzwww/papership-client/model/space"
)

// withParent 给夹具补父/子关系 (单元测试里 assembly 未运行).
func withParent(d document.Document, parent string, children []string) document.Document {
	d.Parent = parent
	d.Children = children
	return d
}

// withTitleLang 给夹具补标题与语言 (normalizer 未运行).
func withTitleLang(d document.Document, title, lang string) document.Document {
	d.Title = title
	d.Lang = lang
	return d
}

// renderFixture 构造 derive 结果: 首页 + 一个段 (guide/README + intro + intro.zh).
func renderFixture(t *testing.T) *derive.Result {
	t.Helper()
	sp := &space.Space{Root: "/tmp/site"}
	index := withParent(withTitleLang(document.Document{Document: testdoc.ParsedDoc("# 首页\n\n欢迎.\n", "docs/index.md", "", "index", true)}, "首页", ""), "", nil)
	guide := withParent(withTitleLang(document.Document{Document: testdoc.ParsedDoc("# 指南\n\n到此一游.\n", "docs/guide/README.md", "guide", "README", true)}, "指南", ""), "docs/index.md", nil)
	intro := withParent(withTitleLang(document.Document{Document: testdoc.ParsedDoc("# 入门\n\n这是正文.\n", "docs/guide/intro.md", "guide", "intro", false)}, "入门", ""), "docs/guide/README.md", nil)
	introZh := withParent(withTitleLang(document.Document{Document: testdoc.ParsedDoc("# 入门中文\n\n中文正文.\n", "docs/guide/intro.zh.md", "guide", "intro", false)}, "入门中文", "zh"), "docs/guide/README.md", nil)
	// 变体关系 (assembly 投影).
	variants := []string{"docs/guide/intro.md", "docs/guide/intro.zh.md"}
	intro.Variants = variants
	introZh.Variants = variants
	// 图谱投影手动补齐.
	index.Children = []string{"docs/guide/README.md"}
	guide.Children = []string{"docs/guide/intro.md", "docs/guide/intro.zh.md"}

	res, err := derive.Derive(context.Background(), &pipeline.Result[document.Document]{
		Space: sp,
		Docs:  []document.Document{index, guide, intro, introZh},
	})
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	return res
}

// mapfs 构造一个虚拟主题 (fstest.MapFS).
func mapfs(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for p, c := range files {
		m[p] = &fstest.MapFile{Data: []byte(c)}
	}
	return m
}

// TestRenderThemeLayout 锁定主题布局: 模板被执行, Markdown 正文原样注入 (不作二次转义),
// 面包屑/上下一篇可用, OutPath 正确.
func TestRenderThemeLayout(t *testing.T) {
	derived := renderFixture(t)
	fsys := mapfs(map[string]string{
		"themes/fancy/layouts/page.html": "PAGE:[{{.Title}}]|URL:{{.URL}}|BODY:{{.Content}}|CRUMB:{{range .Breadcrumb}}{{.URL}}={{.Title}};{{end}}|NEXT:{{.Next.Title}}@{{.Next.URL}}",
	})
	res, err := RenderWithOptions(context.Background(), derived, Options{Theme: "fancy", FS: fsys})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if res.Theme != "fancy" {
		t.Errorf("Theme = %q, want fancy", res.Theme)
	}
	if len(res.Problems) != 0 {
		t.Errorf("Problems = %+v, want 0", res.Problems)
	}

	// 普通页: 正文是渲染后的 Markdown (h1 不应被转义), 面包屑含 首页/指南.
	p := testutilFind(t, res, "docs/guide/intro.md")
	html := string(p.HTML)
	if !strings.Contains(html, "PAGE:[入门]") || !strings.Contains(html, "<h1>入门</h1>") {
		t.Errorf("page HTML = %q, want 标题与渲染正文", html)
	}
	if !strings.Contains(html, "/=首页;") || !strings.Contains(html, "/guide/=指南;") {
		t.Errorf("page 面包屑缺失: %q", html)
	}
	if p.OutPath != "guide/intro/index.html" {
		t.Errorf("OutPath = %q, want guide/intro/index.html", p.OutPath)
	}

	// 上一页 = intro.zh?
	// 顺序: children [intro.md, intro.zh.md] → intro.next = intro.zh.
	if !strings.Contains(html, "NEXT:入门中文@/zh/guide/intro/") {
		t.Errorf("page Next = 缺失: %q", html)
	}
}

// TestRenderKindLayouts 锁定 kind 布局: 根入口 index.html / 段入口 section.html
// 普通页 page.html.
func TestRenderKindLayouts(t *testing.T) {
	derived := renderFixture(t)
	fsys := mapfs(map[string]string{
		"themes/fancy/layouts/index.html":   "KIND:INDEX",
		"themes/fancy/layouts/section.html": "KIND:SECTION",
		"themes/fancy/layouts/page.html":    "KIND:PAGE",
	})
	res, err := RenderWithOptions(context.Background(), derived, Options{Theme: "fancy", FS: fsys})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := map[string]string{
		"docs/index.md":          "KIND:INDEX",
		"docs/guide/README.md":   "KIND:SECTION",
		"docs/guide/intro.md":    "KIND:PAGE",
		"docs/guide/intro.zh.md": "KIND:PAGE",
	}
	for rel, kw := range want {
		p := testutilFind(t, res, rel)
		if string(p.HTML) != kw {
			t.Errorf("%s HTML = %q, want %q", rel, p.HTML, kw)
		}
	}
}

// TestRenderLayoutFallback 锁定兜底: _default.html 命中 / 无布局或主题缺失 → 内置布局;
// 主题缺失产生 warning 问题.
func TestRenderLayoutFallback(t *testing.T) {
	t.Run("_default.html 命中", func(t *testing.T) {
		derived := renderFixture(t)
		fsys := mapfs(map[string]string{"themes/fancy/layouts/_default.html": "DEFAULT:{{.Title}}"})
		res, err := RenderWithOptions(context.Background(), derived, Options{Theme: "fancy", FS: fsys})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if p := testutilFind(t, res, "docs/index.md"); string(p.HTML) != "DEFAULT:首页" {
			t.Errorf("index HTML = %q, want DEFAULT:首页", p.HTML)
		}
	})
	t.Run("主题缺失 → 内置布局 + warning", func(t *testing.T) {
		derived := renderFixture(t)
		res, err := RenderWithOptions(context.Background(), derived, Options{Theme: "ghost", FS: fstest.MapFS{}})
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if !strings.Contains(string(testutilFind(t, res, "docs/index.md").HTML), "<!DOCTYPE html>") {
			t.Error("内置布局未生效")
		}
		found := false
		for _, p := range res.Problems {
			if p.Stage == "render" && p.Severity == pipeline.SeverityWarning {
				found = true
			}
		}
		if !found {
			t.Errorf("主题缺失未产生 warning 问题: %+v", res.Problems)
		}
	})
}

// TestRenderBrokenTemplate 锁定布局语法错误容错: 记入 render error 问题, 页面跳过不崩溃.
func TestRenderBrokenTemplate(t *testing.T) {
	derived := renderFixture(t)
	fsys := mapfs(map[string]string{"themes/fancy/layouts/page.html": "{{ .Broken"})
	res, err := RenderWithOptions(context.Background(), derived, Options{Theme: "fancy", FS: fsys})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if res.DocCount() != 2 {
		t.Errorf("DocCount = %d, want 2 (根+段可用, 两个普通页因坏布局跳过)", res.DocCount())
	}
	hasProblem := false
	for _, p := range res.Problems {
		if p.Stage == "render" && p.Severity == pipeline.SeverityError {
			hasProblem = true
		}
	}
	if !hasProblem {
		t.Errorf("模板错误未收集为 render 问题: %+v", res.Problems)
	}
}

// TestRenderOutPath 锁定输出路径: 根 / 段 / 普通页 / 语言前缀页.
func TestRenderOutPath(t *testing.T) {
	derived := renderFixture(t)
	res, err := Render(context.Background(), derived) // 无主题 → 内置布局.
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := map[string]string{
		"docs/index.md":          "index.html",
		"docs/guide/README.md":   "guide/index.html",
		"docs/guide/intro.md":    "guide/intro/index.html",
		"docs/guide/intro.zh.md": "zh/guide/intro/index.html",
	}
	for rel, w := range want {
		p := testutilFind(t, res, rel)
		if p.OutPath != w {
			t.Errorf("%s OutPath = %q, want %q", rel, p.OutPath, w)
		}
	}
}

// TestRenderNilInput 锁定 nil 输入报错.
func TestRenderNilInput(t *testing.T) {
	if _, err := Render(nil, nil); err == nil {
		t.Fatal("Render(nil): expected error, got nil")
	}
}

// TestRenderStage 锁定 Stage 冒烟.
func TestRenderStage(t *testing.T) {
	res, err := (Stage{}).Run(context.Background(), renderFixture(t))
	if err != nil {
		t.Fatalf("Stage.Run: %v", err)
	}
	if res.DocCount() != 4 {
		t.Errorf("DocCount = %d, want 4", res.DocCount())
	}
}

// testutilFind 按 RelPath 查找渲染页; 缺失即 Fatal.
func testutilFind(t *testing.T, res *Result, rel string) Page {
	t.Helper()
	for _, p := range res.Docs {
		if p.RelPath == rel {
			return p
		}
	}
	t.Fatalf("页面 %q 缺失", rel)
	return Page{}
}
