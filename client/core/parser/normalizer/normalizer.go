// Package normalizer 负责把由 parser 解析成的元数据标准化.
package normalizer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/emanyzwww/papership-client/core/parser/parser"
	"github.com/emanyzwww/papership-client/model/space"
	"github.com/yuin/goldmark/ast"
)

// Document 是一篇标准化后的文档: 解析结果 + 推导出的标准字段.
//
// 与 parser.Document 分离而不是复用, 是为了保持每层契约独立:
// 未来 normalizer 扩展字段 (如摘要、关键词) 只在这里加, 不影响 parser.
type Document struct {
	Parsed  parser.Document // Parsed 解析层完整结果 (Entry/Meta/AST/Body/Hash).
	RelPath string          // RelPath 相对 Space 根目录的路径.
	Dir     string          // Dir 相对 docs 根目录的目录部分; 顶层文档为空.
	Base    string          // Base 剥离语言后缀后的文档基名.
	Lang    string          // Lang 从文件名语言后缀推导的语言码, 如 "zh"; 无后缀为空.
	IsIndex bool            // IsIndex 是否为入口文档 (index / _index / README).
	Title   string          // Title 标题: Front Matter title 优先, 否则取第一个 H1.
	Slug    string          // Slug 用于 URL 的稳定标识, 基于 Base 生成.
}

// Result 是一次 Normalize 的完整产物.
type Result struct {
	Space *space.Space // Space 本次标准化的 Space, 透传.
	Docs  []Document   // Docs 标准化后的文档, 按 RelPath 排序.
}

// DocCount 返回文档数量.
func (r *Result) DocCount() int { return len(r.Docs) }

// Normalize 对一次解析结果执行标准化.
func Normalize(parsed *parser.Result) (*Result, error) {
	if parsed == nil {
		return nil, fmt.Errorf("normalizer: nil parse result")
	}

	res := &Result{Space: parsed.Space}
	for _, d := range parsed.Docs {
		base, lang := splitLang(d.Entry.Stem)
		title := d.Title()
		if title == "" {
			title = firstHeadingTitle(d.AST, d.Body)
		}
		res.Docs = append(res.Docs, Document{
			Parsed:  d,
			RelPath: d.Entry.RelPath,
			Dir:     d.Entry.Dir,
			Base:    base,
			Lang:    lang,
			IsIndex: isIndexName(base),
			Title:   title,
			Slug:    slugify(base),
		})
	}

	sortDocs(res.Docs)
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

// sortDocs 按 RelPath 排序, 与 scanner/parser 的约定一致.
func sortDocs(docs []Document) {
	sort.Slice(docs, func(i, j int) bool { return docs[i].RelPath < docs[j].RelPath })
}
