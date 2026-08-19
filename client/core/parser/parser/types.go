// Package parser 负责将 Markdown 文件解析成 mdast 和元数据.
//
// 它是扫描层之后的第二层: 输入 scanner.Result, 产出本层 Result.
package parser

import (
	"github.com/emanyzwww/papership-client/core/scanner/scanner"
	"github.com/emanyzwww/papership-client/model/space"
	"github.com/yuin/goldmark/ast"
)

// Document 是一篇解析完成的文档: 元数据 + AST + 原始体.
//
// 字段设计遵循 "只加不改" 的契约原则:
// 在 Document 上新增字段即可, 已有字段语义不变.
type Document struct {
	Entry scanner.DocEntry // Entry 来源条目, 带全量路径与元数据.
	Meta  map[string]any   // Meta 标准化前的 Front Matter 值; 无/坏元数据时为空 map.
	AST   *ast.Document    // AST 整篇文档的 goldmark 语法树, 供下游遍历标题/链接等.
	Body  []byte           // Body 去除 Front Matter 后的 Markdown 原文.
	Hash  string           // Hash 原始文件内容 SHA-256, 供后续增量/缓存比对.
}

// Title 返回 Front Matter 中的 title, 否则返回空串.
func (d *Document) Title() string {
	if v, ok := d.Meta["title"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// Result 是一次 Parse 的完整产物: 解析后的文档索引 + 收集到的问题.
//
// Problems 直接复用 scanner.Problem 类型: 从扫描到解析再到后续各层,
// 全管线共享同一种问题形态 (Severity/Path/Message), 便于逐层汇总与展示.
type Result struct {
	Space    *space.Space      // Space 本次解析的 Space, 透传自 scanner.Result.
	Docs     []Document        // Docs 解析后的文档, 按 RelPath 排序.
	Problems []scanner.Problem // Problems 解析中收集的问题.
}

// DocCount 返回文档数量.
func (r *Result) DocCount() int { return len(r.Docs) }
