package assembly

import (
	"testing"

	"github.com/emanyzwww/papership-client/core/assembly/document"
	"github.com/emanyzwww/papership-client/core/parser/parser"
	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/emanyzwww/papership-client/model/space"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// mdDoc 构造一篇解析完成的文档 (AST 由 goldmark 解析 body 得到).
func mdDoc(body, rel, dir, base string, isIndex bool) parser.Document {
	node := goldmark.New().Parser().Parse(text.NewReader([]byte(body)))
	doc, _ := node.(*ast.Document)
	if doc == nil {
		doc = ast.NewDocument()
	}
	return parser.Document{
		Doc:  pipeline.Doc{RelPath: rel, Dir: dir, Base: base, IsIndex: isIndex},
		AST:  doc,
		Body: []byte(body),
	}
}

func findDoc(t *testing.T, res *pipeline.Result[document.Document], rel string) document.Document {
	t.Helper()
	for _, d := range res.Docs {
		if d.RelPath == rel {
			return d
		}
	}
	t.Fatalf("文档 %q 缺失", rel)
	return document.Document{}
}

func hasProblem(t *testing.T, res *pipeline.Result[document.Document], path, severity string) bool {
	t.Helper()
	for _, p := range res.Problems {
		if p.Path == path && p.Severity == pipeline.Severity(severity) {
			return true
		}
	}
	return false
}

// TestAssembleBuildsHierarchy 锁定组装后的文档模型携带层次/变体投影, 顺序与 Space 透传正确.
func TestAssembleBuildsHierarchy(t *testing.T) {
	in := &pipeline.Result[parser.Document]{
		Space: &space.Space{Root: "/tmp/site"},
		Docs: []parser.Document{
			mdDoc("# 首页\n", "docs/index.md", "", "index", true),
			mdDoc("# Guide\n", "docs/guide/README.md", "guide", "README", true),
			mdDoc("# Intro\n", "docs/guide/intro.md", "guide", "intro", false),
			mdDoc("# 介绍\n", "docs/guide/intro.zh.md", "guide", "intro", false),
			mdDoc("# About\n", "docs/about.md", "", "about", false),
		},
	}
	res, err := Assemble(in)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if res.Space != in.Space {
		t.Error("Space 未透传")
	}
	if res.DocCount() != 5 {
		t.Fatalf("DocCount = %d, want 5", res.DocCount())
	}

	idx := findDoc(t, res, "docs/index.md")
	if idx.Parent != "" || len(idx.Children) != 2 {
		t.Errorf("index: Parent=%q Children=%v", idx.Parent, idx.Children)
	}
	guide := findDoc(t, res, "docs/guide/README.md")
	if guide.Parent != "docs/index.md" {
		t.Errorf("guide.Parent = %q, want docs/index.md", guide.Parent)
	}
	intro := findDoc(t, res, "docs/guide/intro.md")
	if intro.Parent != "docs/guide/README.md" {
		t.Errorf("intro.Parent = %q, want docs/guide/README.md", intro.Parent)
	}
	if len(intro.Variants) != 2 {
		t.Errorf("intro.Variants = %v, want 2 (intro.md + intro.zh.md)", intro.Variants)
	}

	for i := 1; i < len(res.Docs); i++ {
		if res.Docs[i-1].RelPath > res.Docs[i].RelPath {
			t.Errorf("docs not sorted: %s > %s", res.Docs[i-1].RelPath, res.Docs[i].RelPath)
		}
	}
}

// TestAssembleResolvesLinks 锁定内部链接解析: 相对路径 / 跨目录 / 外链与锚点忽略,
// 断链收集为 assembly 级 warning 问题且文档照常产出.
func TestAssembleResolvesLinks(t *testing.T) {
	in := &pipeline.Result[parser.Document]{
		Space: &space.Space{Root: "/tmp/site"},
		Docs: []parser.Document{
			mdDoc("# Home\n", "docs/index.md", "", "index", true),
			mdDoc("# Intro\n\n[本目录](intro.zh.md) [上级](../index.md) [外部](https://example.com/x) [锚点](#sec) [断链](missing.md)\n",
				"docs/guide/intro.md", "guide", "intro", false),
			mdDoc("# 介绍\n", "docs/guide/intro.zh.md", "guide", "intro", false),
		},
	}
	res, err := Assemble(in)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	intro := findDoc(t, res, "docs/guide/intro.md")
	want := []string{"docs/guide/intro.zh.md", "docs/index.md"} // 字典序: 'g' < 'i'.
	got := intro.Links
	if len(got) != len(want) {
		t.Fatalf("intro.Links = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("intro.Links[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	idx := findDoc(t, res, "docs/index.md")
	if len(idx.Referrers) != 1 || idx.Referrers[0] != "docs/guide/intro.md" {
		t.Errorf("index.Referrers = %v, want [docs/guide/intro.md]", idx.Referrers)
	}

	if !hasProblem(t, res, "docs/guide/intro.md", "warning") {
		t.Error("断链未收集为 warning 问题")
	}
	if len(res.Problems) != 1 {
		t.Errorf("Problems = %d, want 1", len(res.Problems))
	}
}

// TestAssembleDirectoryLink 锁定目录链接解析: [指南](guide/) 指向该目录入口文档,
// 不再产生假断链警告.
func TestAssembleDirectoryLink(t *testing.T) {
	in := &pipeline.Result[parser.Document]{
		Space: &space.Space{Root: "/tmp/site"},
		Docs: []parser.Document{
			mdDoc("# Home\n\n[指南](guide/)\n", "docs/index.md", "", "index", true),
			mdDoc("# Guide\n", "docs/guide/README.md", "guide", "README", true),
		},
	}
	res, err := Assemble(in)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	idx := findDoc(t, res, "docs/index.md")
	if len(idx.Links) != 1 || idx.Links[0] != "docs/guide/README.md" {
		t.Errorf("index.Links = %v, want [docs/guide/README.md] (目录链接应指向入口文档)", idx.Links)
	}
	if len(res.Problems) != 0 {
		t.Errorf("Problems = %d, want 0 (目录链接不该报断链)", len(res.Problems))
	}
}

// TestAssembleCustomLayout 锁定自定义文档目录布局: 链接解析跟随 Space.Layout.Docs.
func TestAssembleCustomLayout(t *testing.T) {
	in := &pipeline.Result[parser.Document]{
		Space: &space.Space{
			Root:   "/tmp/site",
			Layout: space.Layout{Docs: "content", Themes: "skins"},
		},
		Docs: []parser.Document{
			mdDoc("# Intro\n\n[中文版](intro.zh.md)\n", "content/guide/intro.md", "guide", "intro", false),
			mdDoc("# 介绍\n", "content/guide/intro.zh.md", "guide", "intro", false),
		},
	}
	res, err := Assemble(in)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	intro := findDoc(t, res, "content/guide/intro.md")
	want := []string{"content/guide/intro.zh.md"}
	if len(intro.Links) != 1 || intro.Links[0] != want[0] {
		t.Errorf("intro.Links = %v, want %v (自定义布局下链接应解析为 content/...)",
			intro.Links, want)
	}
	if len(res.Problems) != 0 {
		t.Errorf("Problems = %d, want 0 (自定义布局下不该误报断链)", len(res.Problems))
	}
}

// TestAssembleNilInput 锁定 nil 输入报错.
func TestAssembleNilInput(t *testing.T) {
	if _, err := Assemble(nil); err == nil {
		t.Fatal("Assemble(nil): expected error, got nil")
	}
}

// TestAssembleOnlyOwnProblems 锁定本层只报本阶段问题: 上游问题不进入本层 Result.Problems.
func TestAssembleOnlyOwnProblems(t *testing.T) {
	in := &pipeline.Result[parser.Document]{
		Space: &space.Space{Root: "/tmp/site"},
		Docs:  []parser.Document{mdDoc("# A\n", "docs/a.md", "", "a", false)},
		Problems: []pipeline.Problem{
			{Severity: pipeline.SeverityError, Stage: "parser", Path: "docs/a.md", Message: "上游问题"},
		},
	}
	res, err := Assemble(in)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if len(res.Problems) != 0 {
		t.Errorf("Problems = %d, want 0 (上游问题不该被本层携带)", len(res.Problems))
	}
}
