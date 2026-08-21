package parser

import (
	"testing"

	"github.com/emanyzwww/papership-client/core/markdown"
	"github.com/yuin/goldmark/ast"
)

// firstH1 提取 AST 中第一个一级标题的文本, 供断言使用.
func firstH1(t *testing.T, doc *ast.Document, source []byte) string {
	t.Helper()
	var title string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if h, ok := n.(*ast.Heading); ok && h.Level == 1 {
			title = string(h.Text(source))
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return title
}

// TestParseMarkdownEmpty 锁定空正文: 仍返回非 nil 文档节点, 不 panic.
func TestParseMarkdownEmpty(t *testing.T) {
	doc := parseMarkdown(nil)
	if doc == nil {
		t.Fatal("parseMarkdown(nil) = nil, want non-nil")
	}
}

// TestParseMarkdownHeading 锁定标题结构: H1 文本可提取, H2 不是 H1.
func TestParseMarkdownHeading(t *testing.T) {
	source := []byte("# Hello\n\n## Sub\n\n正文\n")
	doc := parseMarkdown(source)
	if got := firstH1(t, doc, source); got != "Hello" {
		t.Errorf("H1 = %q, want %q", got, "Hello")
	}
}

// TestParseMarkdownInline 锁定行内样式: 强调符号不出现在标题文本中.
func TestParseMarkdownInline(t *testing.T) {
	source := []byte("# Hello *World*\n")
	doc := parseMarkdown(source)
	if got := firstH1(t, doc, source); got != "Hello World" {
		t.Errorf("H1 = %q, want %q (行内强调不携带星号)", got, "Hello World")
	}
}

// TestParseMarkdownDocumentAlwaysRoot 锁定共享包返回非 nil 文档.
func TestParseMarkdownDocumentAlwaysRoot(t *testing.T) {
	if doc := markdown.Parse([]byte("para one\n\npara two\n")); doc == nil {
		t.Fatal("markdown.Parse = nil, want 非 nil 文档")
	}
}
