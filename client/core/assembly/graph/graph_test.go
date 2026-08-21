package graph

import (
	"reflect"
	"testing"
)

// TestGraphBuild 锁定图谱构建: 目录层级 (入口文档为段节点)、子节点排序、
// 反向边、语言变体; 输入无序时结果确定.
func TestGraphBuild(t *testing.T) {
	docs := []Doc{
		{RelPath: "docs/guide/intro.zh.md", Dir: "guide", Base: "intro", Links: []string{"docs/index.md", "docs/guide/intro.md"}},
		{RelPath: "docs/guide/intro.md", Dir: "guide", Base: "intro", Links: []string{"docs/guide/intro.zh.md"}},
		{RelPath: "docs/index.md", Dir: "", Base: "index", IsIndex: true},
		{RelPath: "docs/about.md", Dir: "", Base: "about"},
		{RelPath: "docs/guide/README.md", Dir: "guide", Base: "README", IsIndex: true},
	}
	g := Build(docs)
	if g.Len() != 5 {
		t.Fatalf("Len = %d, want 5", g.Len())
	}

	// order 按 RelPath 升序, 与输入顺序无关.
	wantOrder := []string{
		"docs/about.md",
		"docs/guide/README.md",
		"docs/guide/intro.md",
		"docs/guide/intro.zh.md",
		"docs/index.md",
	}
	if !reflect.DeepEqual(g.Order(), wantOrder) {
		t.Errorf("Order = %v, want %v", g.Order(), wantOrder)
	}

	// 目录层级: 顶层入口是 docs/index.md; guide 段节点是 README.
	idx, _ := g.Node("docs/index.md")
	if idx.Parent != "" || len(idx.Children) != 2 {
		t.Errorf("index: Parent=%q Children=%v, want empty + [about, guide README]", idx.Parent, idx.Children)
	}
	if !reflect.DeepEqual(idx.Children, []string{"docs/about.md", "docs/guide/README.md"}) {
		t.Errorf("index.Children = %v", idx.Children)
	}
	guide, _ := g.Node("docs/guide/README.md")
	if guide.Parent != "docs/index.md" {
		t.Errorf("guide README.Parent = %q, want docs/index.md", guide.Parent)
	}
	if !reflect.DeepEqual(guide.Children, []string{"docs/guide/intro.md", "docs/guide/intro.zh.md"}) {
		t.Errorf("guide.Children = %v", guide.Children)
	}
	intro, _ := g.Node("docs/guide/intro.md")
	if intro.Parent != "docs/guide/README.md" {
		t.Errorf("intro.Parent = %q, want docs/guide/README.md", intro.Parent)
	}

	// 反向边.
	if !reflect.DeepEqual(idx.Referrers, []string{"docs/guide/intro.zh.md"}) {
		t.Errorf("index.Referrers = %v", idx.Referrers)
	}

	// 语言变体 (含自身).
	if !reflect.DeepEqual(intro.Variants, []string{"docs/guide/intro.md", "docs/guide/intro.zh.md"}) {
		t.Errorf("intro.Variants = %v", intro.Variants)
	}
	if len(idx.Variants) != 0 {
		t.Errorf("index.Variants = %v, want empty", idx.Variants)
	}
}
