// Package parser 负责将 Markdown 文件解析成 mdast 和元数据.
//
// 它是扫描层之后的第二层: 输入 scanner.Result, 产出本层 Result.
//
// 文档脊柱 (pipeline.Doc) 由全管线共享: parser 填充物理/内容事实
// (RelPath/Dir/Stem/Ext/Size/ModTime/Hash), 语义字段 (Base/Lang/IsIndex/Title/Slug)
// 留给 normalizer 推导; 本层只把元数据与语法树作为载荷挂在 Document 上.
package parser

import (
	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/yuin/goldmark/ast"
)

// Document 是一篇解析完成的文档: 共享脊柱 + 本层载荷.
//
// 字段设计遵循 "只加不改" 的契约原则: 在 Document 上新增字段即可,
// 已有字段语义不变 (脊柱字段见 pipeline.Doc).
type Document struct {
	pipeline.Doc                // 脊柱: 排序键, 路径, 大小, 修改时间, 内容哈希等.
	Meta         map[string]any // Meta 标准化前的 Front Matter 值; 无/坏元数据时为空 map.
	AST          *ast.Document  // AST 整篇文档的 goldmark 语法树, 供下游遍历标题/链接等.
	Body         []byte         // Body 去除 Front Matter 后的 Markdown 原文.
}

// MetaTitle 返回 Front Matter 中的 title, 否则返回空串.
//
// 注意: 这是 content 层的原始查询; 标准化标题 (FM 优先, H1 兜底) 由
// normalizer 写入共享脊柱的 Doc.Title 字段, 下游读取字段而非本方法.
// 方法不叫 Title 是为了避免与共享脊柱的 Title 字段同名.
func (d *Document) MetaTitle() string {
	if v, ok := d.Meta["title"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// Result 是一次 Parse 的完整产物; 信封与问题形态直接复用 pipeline.
type Result = pipeline.Result[Document]
