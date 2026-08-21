// Package normalizer 负责把由 parser 解析成的元数据标准化.
//
// 文档类型直接复用 parser.Document: 共享脊柱 (pipeline.Doc) 里已有的
// 物理/内容事实不变, 本层只推导并写入语义字段 (Base/Lang/IsIndex/Title/Slug).
// 这样下游 assembly/derive/render 拿到的是同一个文档类型, 不再为每一层复制一份
// 结构定义.
package normalizer

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/emanyzwww/papership-client/core/parser/parser"
	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/yuin/goldmark/ast"
)

// Document 与 parser.Document 是同一个类型 (别名):
// 标准化不引入新结构, 只填充脊柱语义字段.
type Document = parser.Document

// Result 是一次 Normalize 的完整产物; 信封复用 pipeline.
type Result = pipeline.Result[Document]

// Stage 是标准化阶段: 实现 pipeline.Stage, 供编排层串联; 零值可用.
type Stage struct{}

// Run 执行一次带上下文的标准化.
func (Stage) Run(ctx context.Context, in *parser.Result) (*Result, error) { return Normalize(ctx, in) }

// Normalize 对一次解析结果执行标准化, 上下文取消时中止:
// 推导语言/入口/标题/slug 并写入脊柱.
func Normalize(ctx context.Context, parsed *parser.Result) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if parsed == nil {
		return nil, fmt.Errorf("normalizer: nil parse result")
	}

	res := pipeline.NewResult[Document](parsed.Space)
	for _, d := range parsed.Docs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		base, lang := splitLang(d.Stem)
		title := d.MetaTitle() // 方法: 仅查 Front Matter.
		if title == "" {
			title = firstHeadingTitle(d.AST, d.Body)
		}
		doc := d // 拷贝, 在副本上填充语义字段.
		doc.Base = base
		doc.Lang = lang
		doc.IsIndex = isIndexName(base)
		doc.Title = title
		doc.Slug = slugify(base)
		res.Docs = append(res.Docs, doc)
	}

	pipeline.SortByKey(res.Docs)
	return res, nil
}

// langSuffixRE 匹配形如 "intro.zh" / "guide.en-us" 的语言后缀:
// 两位小写字母, 可带 "-xx" 地区后缀. 后缀判定要求小写字母开头,
// 避免把普通文件名里的全大写段 (如 "c.EN") 误判为语言.
var langSuffixRE = regexp.MustCompile("^(.+)\\.([a-z]{2}(?:-[A-Za-z]{2,})?)$")

// splitLang 从文件基名 (已剥离扩展名) 中拆分语言后缀.
//
// 无匹配时返回原基名与空语言.
func splitLang(stem string) (base, lang string) {
	m := langSuffixRE.FindStringSubmatch(stem)
	if m == nil {
		return stem, ""
	}
	return m[1], strings.ToLower(m[2])
}

// isIndexName 判断基名是否为入口文档: index / _index / README (大小写不敏感).
func isIndexName(base string) bool {
	switch strings.ToLower(base) {
	case "index", "_index", "readme":
		return true
	}
	return false
}

// firstHeadingTitle 从文档 AST 中提取第一个一级标题的文本.
//
// h.Text(source) 依赖原始正文还原 AST 中 Text 节点引用的位置, 所以
// 必须传入该文档的 Body; 未找到 H1 时返回空串.
func firstHeadingTitle(doc *ast.Document, source []byte) string {
	var title string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if h, ok := n.(*ast.Heading); ok && h.Level == 1 {
			title = strings.TrimSpace(string(h.Text(source)))
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return title
}

// slugify 把基名转为适合 URL 的 slug:
// 小写化, 保留字母/数字/汉字, 其余字符折叠为单个连字符, 并去掉首尾连字符.
// 中文站点下汉字原样保留, 避免 URL 变成无意义的长转义.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r >= 0x4e00 && r <= 0x9fff:
			b.WriteRune(r)
			dash = false
		default:
			if !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
