// Package document 定义统一文档模型: 解析与标准化全量事实 + 图谱投影.
//
// 本包是纯数据结构, 不依赖 graph 包 (投影由上层的 assembly 填充),
// 保证下游 (derive/render) 只消费本模型即可, 无需感知图谱内部.
package document

import "github.com/emanyzwww/papership-client/core/parser/parser"

// Document 是一份完成组装后的文档模型.
//
// 内嵌 parser.Document 继承解析+标准化的全部事实 (脊柱 + Meta/AST/Body);
// 以下字段是图谱投影: 由 core/assembly 在构建站点图谱后逐篇填充.
type Document struct {
	parser.Document
	Parent    string   // Parent 父文档 RelPath (目录层级); 顶层为空.
	Children  []string // Children 直接子文档 RelPath, 按 RelPath 升序.
	Links     []string // Links 出向内部链接 (已解析为 RelPath, 去重).
	Referrers []string // Referrers 引用本文档的文档 RelPath (反向边).
	Variants  []string // Variants 同 Base 的语言变体 RelPath (含自身); 无变体时为空.
}
