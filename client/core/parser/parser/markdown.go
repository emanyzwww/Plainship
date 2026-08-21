package parser

import (
	"github.com/emanyzwww/papership-client/core/markdown"
	"github.com/yuin/goldmark/ast"
)

// parseMarkdown 把 Markdown 原文解析成 goldmark AST.
//
// goldmark 实例与扩展配置集中在 core/markdown (parser 与 render 共用),
// 这里保留薄包装, 层内调用点不受底层变化影响.
func parseMarkdown(body []byte) *ast.Document {
	return markdown.Parse(body)
}
