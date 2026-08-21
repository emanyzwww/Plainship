// Package testdoc 提供依赖 parser 测试夹具 (需真实 goldmark AST 的场景).
//
// 与 testutil 分开: 避免 parser 的测试 import testutil 时形成
// parser → testutil → parser 的 import 环.
package testdoc

import (
	"github.com/emanyzwww/papership-client/core/parser/parser"
	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// ParsedDoc 构造一篇解析完成的文档: AST 由 goldmark 解析 body 得到.
func ParsedDoc(body, rel, dir, base string, isIndex bool) parser.Document {
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
