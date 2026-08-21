// Package derive 是功能派生层入口: 基于站点图谱产出导航, URL, 站点地图与搜索索引等派生数据.
//
// 输入 assembly 的统一文档模型 (document.Document), 输出本层的派生结果:
//   - 每页: clean URL (写入共享脊柱 pipeline.Doc.URL), 面包屑, 同段上/下篇;
//   - 全局: 导航树, 全部 URL 清单 (sitemap), 搜索索引.
//
// 与上游一致, 本层只报本阶段问题; 当前派生规则均为确定性转换, 不产生问题.
package derive

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/emanyzwww/papership-client/core/assembly/document"
	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/yuin/goldmark/ast"
)

// NavItem 是导航树的一个节点.
type NavItem struct {
	Title    string     // Title 展示名: 标题优先, 否则基名.
	URL      string     // URL 输出路径.
	Children []*NavItem // Children 子节点, 按 RelPath 字典序.
}

// Page 是派生后的页面: 统一文档模型 + 派生上下文.
//
// URL 不再重复声明: 提升自共享脊柱 (pipeline.Doc.URL), 派生时直接写入脊柱,
// 下游 (render/output) 从同一字段读取, 保证一致.
type Page struct {
	document.Document
	Nav     []NavItem // Nav 面包屑: 根 → 直接父 (不含自身).
	Section string    // Section 所属段标题 (直接父的标题; 顶层为空).
	Prev    string    // Prev 同段前一篇的 URL; 无则空.
	Next    string    // Next 同段后一篇的 URL; 无则空.
}

// SearchEntry 是搜索索引中的一条.
type SearchEntry struct {
	URL   string // URL 输出路径.
	Title string // Title 页面标题.
	Text  string // Text 页面正文纯文本 (空白折叠).
}

// Result 是一次 Derive 的完整产物.
type Result struct {
	pipeline.Result[Page]               // Pages + Space + Problems (本阶段无问题).
	Nav                   []NavItem     // Nav 全局导航树 (顶层节点).
	SiteMap               []string      // SiteMap 全部页面输出 URL, 排序去重.
	SearchIndex           []SearchEntry // SearchIndex 搜索索引.
}

// Stage 是派生阶段: 实现 pipeline.Stage, 供编排层串联; 零值可用.
type Stage struct{}

// Run 执行一次带上下文的派生.
func (Stage) Run(ctx context.Context, in *pipeline.Result[document.Document]) (*Result, error) {
	return Derive(ctx, in)
}

// Derive 对组装结果执行派生: URL / 面包屑 / 上下一篇 / 导航树 / 站点地图 / 搜索索引.
func Derive(ctx context.Context, docs *pipeline.Result[document.Document]) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if docs == nil {
		return nil, fmt.Errorf("derive: nil input")
	}

	docsRoot := "docs"
	if docs.Space != nil && docs.Space.Layout.Docs != "" {
		docsRoot = docs.Space.Layout.Docs
	}
	siteLang := ""
	if docs.Space != nil {
		siteLang = docs.Space.Config.SiteLanguage
	}

	// 变体组默认语言: 组内默认变体 (站点语言或 lang 为空) 的 URL 不带语言前缀.
	byDoc := make(map[string]document.Document, docs.DocCount())
	for _, d := range docs.Docs {
		byDoc[d.RelPath] = d
	}
	groupDefault := variantDefaults(docs.Docs, byDoc, siteLang)

	// 第一遍: 建索引, 一次性计算 URL 并写回共享脊柱.
	pages := make([]Page, 0, docs.DocCount())
	byRel := make(map[string]Page, docs.DocCount())
	for _, d := range docs.Docs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pg := Page{Document: d}
		pg.URL = urlFor(pg, docsRoot, groupDefault[groupKeyOf(d.Dir, d.Base)]) // 写回共享脊柱.
		byRel[d.RelPath] = pg
		pages = append(pages, pg)
	}

	// 第二遍: 填派生上下文 (只读 byRel, URL 均为定值).
	for i := range pages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pg := &pages[i]
		sec := ""
		if pg.Parent != "" {
			if par, ok := byRel[pg.Parent]; ok {
				sec = PageTitle(par)
			}
		}
		pg.Section = sec
		pg.prevNext(byRel)
		pg.breadcrumb(byRel)
	}

	// 全局派生物.
	nav := buildNav(pages)
	siteMap := siteMapOf(pages)
	search := searchIndex(pages)

	return &Result{
		Result:      pipeline.Result[Page]{Space: docs.Space, Docs: pages},
		Nav:         nav,
		SiteMap:     siteMap,
		SearchIndex: search,
	}, nil
}

// ==============================
// URL 派生.
// ==============================

// urlFor 派生页面输出路径 (clean URL).
//
// 规则:
//   - 去掉文档根前缀与扩展名, 路径段用 Base/Slug (剥离语言后缀):
//     docs/guide/intro.zh.md → /guide/intro/;
//   - 入口文档 → 目录 URL: docs/index.md → /, docs/guide/README.md → /guide/;
//   - 语言变体: 组内默认语言不带前缀, 其余变体带 /<lang>/ 前缀,
//     避免同基名多语言 URL 冲突.
func urlFor(p Page, docsRoot, defaultLang string) string {
	rel := strings.TrimPrefix(p.RelPath, docsRoot+"/")
	for _, ext := range []string{".markdown", ".md"} {
		if strings.HasSuffix(rel, ext) {
			rel = strings.TrimSuffix(rel, ext)
			break
		}
	}
	name := p.Slug
	if name == "" {
		name = p.Base // Slug 未推导时兜底 (正常由 normalizer 填充).
	}
	var base string
	dir := path.Dir(rel)
	switch {
	case p.IsIndex && dir == ".":
		base = "/"
	case p.IsIndex:
		base = "/" + dir + "/"
	case dir == ".":
		base = "/" + name + "/"
	default:
		base = "/" + dir + "/" + name + "/"
	}
	if len(p.Variants) > 1 && p.Lang != "" && p.Lang != defaultLang {
		return "/" + p.Lang + base
	}
	return base
}

// groupKeyOf 是变体分组键: "目录/基名"; 与 assembly/graph 的分组一致.
func groupKeyOf(dir, base string) string {
	return strings.TrimSuffix(dir+"/", "/") + "/" + base
}

// variantDefaults 计算每个变体组的默认语言:
// lang 为空的变体恒为默认; 否则站点默认语言命中优先; 再否则取 RelPath 最小变体的语言.
func variantDefaults(docs []document.Document, byDoc map[string]document.Document, siteLang string) map[string]string {
	out := make(map[string]string)
	for _, d := range docs {
		if len(d.Variants) < 2 {
			continue
		}
		key := groupKeyOf(d.Dir, d.Base)
		if _, ok := out[key]; ok {
			continue
		}
		def := ""
		hasEmpty := false
		for _, v := range d.Variants {
			if byDoc[v].Lang == "" {
				hasEmpty = true
				break
			}
		}
		if !hasEmpty {
			def = siteLang
			if def == "" && len(d.Variants) > 0 {
				def = byDoc[d.Variants[0]].Lang
			}
		}
		out[key] = def
	}
	return out
}

// ==============================
// 导航派生.
// ==============================

// PageTitle 派生展示名: 标题优先, 否则基名; 供下游层 (render) 复用.
func PageTitle(p Page) string {
	if p.Title != "" {
		return p.Title
	}
	return p.Base
}

// breadcrumb 派生面包屑: 根 → 直接父 (不含自身).
func (p *Page) breadcrumb(byRel map[string]Page) {
	var chain []NavItem
	cur := *p
	for cur.Parent != "" {
		par, ok := byRel[cur.Parent]
		if !ok {
			break
		}
		chain = append(chain, NavItem{Title: PageTitle(par), URL: par.URL})
		cur = par
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	p.Nav = chain
}

// prevNext 派生同段上/下篇 (直接父的 Children 序列).
func (p *Page) prevNext(byRel map[string]Page) {
	if p.Parent == "" {
		return
	}
	par, ok := byRel[p.Parent]
	if !ok {
		return
	}
	for i, c := range par.Children {
		if c != p.RelPath {
			continue
		}
		if i > 0 {
			if sib, ok := byRel[par.Children[i-1]]; ok {
				p.Prev = sib.URL
			}
		}
		if i+1 < len(par.Children) {
			if sib, ok := byRel[par.Children[i+1]]; ok {
				p.Next = sib.URL
			}
		}
		return
	}
}

// buildNav 构建全局导航树: 顶层节点为无父节点者, 子节点递归挂载.
func buildNav(pages []Page) []NavItem {
	byRel := make(map[string]Page, len(pages))
	for _, p := range pages {
		byRel[p.RelPath] = p
	}
	var roots []NavItem
	for _, p := range pages {
		if p.Parent == "" {
			roots = append(roots, *nodeItem(p, byRel))
		}
	}
	return roots
}

// nodeItem 递归构造导航节点.
func nodeItem(p Page, byRel map[string]Page) *NavItem {
	it := &NavItem{Title: PageTitle(p), URL: p.URL}
	for _, c := range p.Children {
		if cp, ok := byRel[c]; ok {
			it.Children = append(it.Children, nodeItem(cp, byRel))
		}
	}
	return it
}

// siteMapOf 汇总全部页面 URL, 排序.
func siteMapOf(pages []Page) []string {
	out := make([]string, 0, len(pages))
	for _, p := range pages {
		out = append(out, p.URL)
	}
	sort.Strings(out)
	return out
}

// ==============================
// 搜索索引.
// ==============================

// searchIndex 生成搜索索引: 每个页面的 URL / 标题 / 正文纯文本.
func searchIndex(pages []Page) []SearchEntry {
	out := make([]SearchEntry, 0, len(pages))
	for _, p := range pages {
		out = append(out, SearchEntry{
			URL:   p.URL,
			Title: PageTitle(p),
			Text:  plainText(p.AST, p.Body),
		})
	}
	return out
}

// plainText 从 goldmark AST 提取正文纯文本并折叠空白.
func plainText(doc *ast.Document, source []byte) string {
	if doc == nil {
		return ""
	}
	var b strings.Builder
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.Text:
			b.Write(t.Value(source))
		case *ast.String:
			b.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return strings.Join(strings.Fields(b.String()), " ")
}
