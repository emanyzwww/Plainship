package parser

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// newMarkdown 集中初始化本层唯一的 goldmark 实例.
//
// 这是 parser 层最重要的扩展缝:
// 未来需要 GFM 表格/脚注/代码高亮等能力时,
// 只需要在这里加 goldmark.WithExtensions(...) 注册对应扩展, 整个层其它代码零改动.
// 所有 parseMarkdown 调用共享同一实例, 保证行为一致.
func newMarkdown() goldmark.Markdown {
	return goldmark.New()
}

// parseMarkdown 把 Markdown 原文解析成 goldmark AST.
//
// 这里直接使用 Parser().Parse 拿到语法树, 渲染层后续基于同一 AST 生成 HTML.
//
// 保留 AST 也就保留了标题/链接/代码块等结构化信息, 供 normalizer/derive 遍历.
func parseMarkdown(body []byte) *ast.Document {
	node := newMarkdown().Parser().Parse(text.NewReader(body))
	if doc, ok := node.(*ast.Document); ok {
		return doc
	}
	// goldmark 的顶层节点恒为 *ast.Document; 这里做防御,
	// 避免空文档时返回 nil 导致下游 panic.
	return ast.NewDocument()
}
