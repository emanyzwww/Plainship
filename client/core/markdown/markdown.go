// Package markdown 集中管理项目唯一的 goldmark 实例: parser 解析与 render 渲染共用同一配置.
//
// 这是 Markdown 能力的唯一扩展缝: 新增 GFM 表格/脚注/代码高亮等,
// 只需在这里注册 goldmark 扩展, parser 与 render 零改动.
package markdown

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// md 是项目唯一的 goldmark 实例.
var md = goldmark.New()

// Parse 把 Markdown 原文解析成 goldmark AST.
//
// 顶层节点恒为 *ast.Document; 空/异常输入返回非 nil 空文档, 避免下游 panic.
func Parse(body []byte) *ast.Document {
	node := md.Parser().Parse(text.NewReader(body))
	if doc, ok := node.(*ast.Document); ok {
		return doc
	}
	return ast.NewDocument()
}

// RenderHTML 把 AST 渲染回 HTML; 基于 source (原文) 还原文本段.
//
// 渲染失败 (如 nil 文档) 返回 nil, 调用方按空内容处理.
func RenderHTML(doc *ast.Document, source []byte) []byte {
	if doc == nil {
		return nil
	}
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, source, doc); err != nil {
		return nil
	}
	return buf.Bytes()
}
