package derive

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/emanyzwww/papership-client/core/assembly/document"
	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/emanyzwww/papership-client/internal/testdoc"
	"github.com/emanyzwww/papership-client/internal/testutil"
	"github.com/emanyzwww/papership-client/model/space"
)

// deriveFixture 把文档装入带 Space 的组装结果, 作为各用例的输入.
func deriveFixture(t *testing.T, docs []document.Document, spaceCfg func(*space.Space)) *pipeline.Result[document.Document] {
	t.Helper()
	sp := &space.Space{Root: "/tmp/site"}
	if spaceCfg != nil {
		spaceCfg(sp)
	}
	return &pipeline.Result[document.Document]{Space: sp, Docs: docs}
}

// newDoc 构造带图谱投影的统一文档模型.
func newDoc(body, rel, dir, base string, isIndex bool, parent string, children []string, variants []string) document.Document {
	d := document.Document{Document: testdoc.ParsedDoc(body, rel, dir, base, isIndex)}
	d.Parent, d.Children, d.Variants = parent, children, variants
	return d
}

// withTitle 给夹具补标题 (单元测试里 normalizer 未运行, 脊柱 Title 为空).
func withTitle(d document.Document, title string) document.Document {
	d.Title = title
	return d
}

// withLang 给夹具补语言 (单元测试里 normalizer 未运行, 脊柱 Lang 为空).
func withLang(d document.Document, lang string) document.Document {
	d.Lang = lang
	return d
}

// TestDeriveURLs 锁定 clean URL 规则: 入口/普通/自定义文档根/语言变体前缀.
func TestDeriveURLs(t *testing.T) {
	docs := []document.Document{
		newDoc("# 首页\n", "docs/index.md", "", "index", true, "", []string{"docs/about.md", "docs/guide/README.md"}, nil),
		newDoc("# About\n", "docs/about.md", "", "about", false, "docs/index.md", nil, nil),
		newDoc("# Guide\n", "docs/guide/README.md", "guide", "README", true, "docs/index.md",
			[]string{"docs/guide/intro.md", "docs/guide/intro.zh.md"}, nil),
		newDoc("# Intro\n", "docs/guide/intro.md", "guide", "intro", false, "docs/guide/README.md", nil,
			[]string{"docs/guide/intro.md", "docs/guide/intro.zh.md"}),
		withLang(newDoc("# 介绍\n", "docs/guide/intro.zh.md", "guide", "intro", false, "docs/guide/README.md", nil,
			[]string{"docs/guide/intro.md", "docs/guide/intro.zh.md"}), "zh"),
	}
	t.Run("默认站点语言为空", func(t *testing.T) {
		res, err := Derive(context.Background(), deriveFixture(t, docs, nil))
		if err != nil {
			t.Fatalf("Derive: %v", err)
		}
		want := map[string]string{
			"docs/index.md":          "/",
			"docs/about.md":          "/about/",
			"docs/guide/README.md":   "/guide/",
			"docs/guide/intro.md":    "/guide/intro/",
			"docs/guide/intro.zh.md": "/zh/guide/intro/",
		}
		checkURLs(t, res, want)
		// URL 写回共享脊柱 (Page.URL 与 Doc.URL 是同一字段).
		if d, ok := testutil.Lookup(res.Docs, "docs/guide/intro.zh.md"); ok {
			if d.Doc.URL != "/zh/guide/intro/" {
				t.Errorf("脊柱 Doc.URL = %q, want /zh/guide/intro/", d.Doc.URL)
			}
			if d.URL != d.Doc.URL {
				t.Errorf("Page.URL = %q != Doc.URL = %q (应提升自同一字段)", d.URL, d.Doc.URL)
			}
		}
	})
	t.Run("站点默认语言为 zh", func(t *testing.T) {
		zhDocs := []document.Document{
			newDoc("# Home\n", "docs/index.md", "", "index", true, "", nil, nil),
			withLang(newDoc("# Intro\n", "docs/guide/intro.en.md", "guide", "intro", false, "", nil,
				[]string{"docs/guide/intro.en.md", "docs/guide/intro.zh.md"}), "en"),
			withLang(newDoc("# 介绍\n", "docs/guide/intro.zh.md", "guide", "intro", false, "", nil,
				[]string{"docs/guide/intro.en.md", "docs/guide/intro.zh.md"}), "zh"),
		}
		res, err := Derive(context.Background(), deriveFixture(t, zhDocs, func(sp *space.Space) {
			sp.Config.SiteLanguage = "zh"
		}))
		if err != nil {
			t.Fatalf("Derive: %v", err)
		}
		want := map[string]string{
			"docs/index.md":          "/",
			"docs/guide/intro.en.md": "/en/guide/intro/", // en 非默认 → 前缀.
			"docs/guide/intro.zh.md": "/guide/intro/",    // zh 是站点默认语言 → 无前缀.
		}
		checkURLs(t, res, want)
	})
}

func checkURLs(t *testing.T, res *Result, want map[string]string) {
	t.Helper()
	for rel, u := range want {
		if d, ok := testutil.Lookup(res.Docs, rel); ok {
			if d.URL != u {
				t.Errorf("%s URL = %q, want %q", rel, d.URL, u)
			}
		} else {
			t.Errorf("页面 %q 缺失", rel)
		}
	}
}

// TestDeriveCustomLayout 锁定自定义文档根: URL 跟随 Space.Layout.Docs.
func TestDeriveCustomLayout(t *testing.T) {
	docs := []document.Document{
		newDoc("# Home\n", "content/index.md", "", "index", true, "", nil, nil),
		newDoc("# Intro\n", "content/guide/intro.md", "guide", "intro", false, "", nil, nil),
	}
	res, err := Derive(context.Background(), deriveFixture(t, docs, func(sp *space.Space) {
		sp.Layout.Docs = "content"
	}))
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	if d, ok := testutil.Lookup(res.Docs, "content/index.md"); ok && d.URL != "/" {
		t.Errorf("content/index.md URL = %q, want /", d.URL)
	}
	if d, ok := testutil.Lookup(res.Docs, "content/guide/intro.md"); ok && d.URL != "/guide/intro/" {
		t.Errorf("intro URL = %q, want /guide/intro/", d.URL)
	}
}

// TestDeriveNav 锁定导航树 / 面包屑 / 上下一篇 / 所属段.
func TestDeriveNav(t *testing.T) {
	docs := []document.Document{
		withTitle(newDoc("# 首页\n", "docs/index.md", "", "index", true, "",
			[]string{"docs/about.md", "docs/guide/README.md"}, nil), "首页"),
		withTitle(newDoc("# About\n", "docs/about.md", "", "about", false, "docs/index.md", nil, nil), "About"),
		withTitle(newDoc("# 指南\n", "docs/guide/README.md", "guide", "README", true, "docs/index.md",
			[]string{"docs/guide/intro.md"}, nil), "指南"),
		withTitle(newDoc("# 入门\n", "docs/guide/intro.md", "guide", "intro", false, "docs/guide/README.md", nil, nil), "入门"),
	}
	res, err := Derive(context.Background(), deriveFixture(t, docs, nil))
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}

	// 全局导航树: 根节点是首页 (唯一无父文档), 含 about 与 指南 两个子节点.
	if len(res.Nav) != 1 || res.Nav[0].URL != "/" || res.Nav[0].Title != "首页" {
		t.Fatalf("Nav 根 = %+v, want [首页 /]", res.Nav)
	}
	root := res.Nav[0]
	if len(root.Children) != 2 {
		t.Fatalf("Nav 根子节点 = %d, want 2 (about + 指南)", len(root.Children))
	}
	children := []string{root.Children[0].Title, root.Children[1].Title}
	wantChildren := []string{"About", "指南"}
	if !reflect.DeepEqual(children, wantChildren) {
		t.Errorf("Nav 子节点标题 = %v, want %v (RelPath 字典序)", children, wantChildren)
	}
	guide := root.Children[1]
	if len(guide.Children) != 1 || guide.Children[0].Title != "入门" {
		t.Errorf("指南子节点 = %+v, want [入门]", guide.Children)
	}

	// 页面级派生上下文.
	intro, _ := testutil.Lookup(res.Docs, "docs/guide/intro.md")
	if len(intro.Nav) != 2 || intro.Nav[0].URL != "/" || intro.Nav[1].URL != "/guide/" {
		t.Errorf("intro.Nav = %+v, want 面包屑 [/ → /guide/]", intro.Nav)
	}
	if intro.Section != "指南" {
		t.Errorf("intro.Section = %q, want 指南", intro.Section)
	}
	if intro.Prev != "" || intro.Next != "" {
		t.Errorf("intro 无兄弟却得到 Prev=%q Next=%q", intro.Prev, intro.Next)
	}
	about, _ := testutil.Lookup(res.Docs, "docs/about.md")
	if about.Section != "首页" {
		t.Errorf("about.Section = %q, want 首页", about.Section)
	}
}

// TestDerivePrevNext 锁定同段上/下篇: 兄弟序列内相邻.
func TestDerivePrevNext(t *testing.T) {
	docs := []document.Document{
		newDoc("# Guide\n", "docs/guide/README.md", "guide", "README", true, "",
			[]string{"docs/guide/a.md", "docs/guide/b.md", "docs/guide/c.md"}, nil),
		newDoc("# A\n", "docs/guide/a.md", "guide", "a", false, "docs/guide/README.md", nil, nil),
		newDoc("# B\n", "docs/guide/b.md", "guide", "b", false, "docs/guide/README.md", nil, nil),
		newDoc("# C\n", "docs/guide/c.md", "guide", "c", false, "docs/guide/README.md", nil, nil),
	}
	res, err := Derive(context.Background(), deriveFixture(t, docs, nil))
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	if d, _ := testutil.Lookup(res.Docs, "docs/guide/a.md"); d.Prev != "" || d.Next != "/guide/b/" {
		t.Errorf("a: Prev=%q Next=%q, want ''/ /guide/b/", d.Prev, d.Next)
	}
	if d, _ := testutil.Lookup(res.Docs, "docs/guide/b.md"); d.Prev != "/guide/a/" || d.Next != "/guide/c/" {
		t.Errorf("b: Prev=%q Next=%q, want /guide/a/ /guide/c/", d.Prev, d.Next)
	}
	if d, _ := testutil.Lookup(res.Docs, "docs/guide/c.md"); d.Prev != "/guide/b/" || d.Next != "" {
		t.Errorf("c: Prev=%q Next=%q, want /guide/b/ ''", d.Prev, d.Next)
	}
}

// TestDeriveSiteMapAndSearch 锁定站点地图与搜索索引.
func TestDeriveSiteMapAndSearch(t *testing.T) {
	docs := []document.Document{
		withTitle(newDoc("# 首页\n", "docs/index.md", "", "index", true, "", nil, nil), "首页"),
		withTitle(newDoc("# 入门\n\n这是正文, 包含 关键词甲 和关键词乙.\n", "docs/guide/intro.md", "guide", "intro", false, "", nil, nil), "入门"),
	}
	res, err := Derive(context.Background(), deriveFixture(t, docs, nil))
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	wantMap := []string{"/", "/guide/intro/"}
	if !reflect.DeepEqual(res.SiteMap, wantMap) {
		t.Errorf("SiteMap = %v, want %v", res.SiteMap, wantMap)
	}
	if len(res.SearchIndex) != 2 {
		t.Fatalf("SearchIndex = %d, want 2", len(res.SearchIndex))
	}
	var intro SearchEntry
	for _, e := range res.SearchIndex {
		if e.URL == "/guide/intro/" {
			intro = e
		}
	}
	if intro.Title != "入门" {
		t.Errorf("SearchEntry.Title = %q, want 入门", intro.Title)
	}
	if !strings.Contains(intro.Text, "关键词甲") || strings.Contains(intro.Text, "\n") {
		t.Errorf("SearchEntry.Text = %q, want 包含关键词甲且空白折叠", intro.Text)
	}
}

// TestDeriveNilInput 锁定 nil 输入报错.
func TestDeriveNilInput(t *testing.T) {
	if _, err := Derive(context.Background(), nil); err == nil {
		t.Fatal("Derive(nil): expected error, got nil")
	}
}

// TestDeriveStage 锁定 Stage 实现可用性 (编译期已由 build 断言, 这里冒烟).
func TestDeriveStage(t *testing.T) {
	res, err := (Stage{}).Run(context.Background(), &pipeline.Result[document.Document]{Space: &space.Space{Root: "/x"}})
	if err != nil {
		t.Fatalf("Stage.Run: %v", err)
	}
	if res.DocCount() != 0 {
		t.Errorf("DocCount = %d, want 0", res.DocCount())
	}
}
