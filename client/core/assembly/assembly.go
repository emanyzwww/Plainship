// Package assembly 是组装层入口: 把解析+标准化完成的文档组装成统一文档模型,
// 并基于全部文档构建站点图谱 (目录层级 / 内部链接 / 语言变体).
//
// 输入 normalizer 的产出, 输出 document.Document: 每篇文档内嵌 parser.Document
// 全量事实, 并携带图谱投影 (Parent/Children/Links/Referrers/Variants).
// 与上游一致, 本层只报本阶段问题, 跨阶段汇总由 core/build 负责.
package assembly

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/emanyzwww/papership-client/core/assembly/document"
	"github.com/emanyzwww/papership-client/core/assembly/graph"
	"github.com/emanyzwww/papership-client/core/parser/parser"
	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/yuin/goldmark/ast"
)

// problem 构造带组装层标记的共享问题.
func problem(sev pipeline.Severity, docPath, format string, args ...any) pipeline.Problem {
	return pipeline.Problemf(sev, "assembly", docPath, format, args...)
}

// Assemble 执行一次组装: 对每篇文档提取内部链接, 构建站点图谱, 再投影为统一文档模型.
//
// 返回的 error 仅代表整层无法继续 (如 nil 输入); 单文档问题进入 Result.Problems.
func Assemble(docs *pipeline.Result[parser.Document]) (*pipeline.Result[document.Document], error) {
	if docs == nil {
		return nil, fmt.Errorf("assembly: nil input")
	}

	known := make(map[string]bool, docs.DocCount())
	for _, d := range docs.Docs {
		known[d.RelPath] = true
	}

	// docs 根目录跟随自定义布局; 链接解析必须与文档 RelPath 前缀一致.
	docsRoot := "docs"
	if docs.Space != nil && docs.Space.Layout.Docs != "" {
		docsRoot = docs.Space.Layout.Docs
	}

	gd := make([]graph.Doc, 0, docs.DocCount())
	var linkProblems []pipeline.Problem
	for _, d := range docs.Docs {
		links, probs := resolveLinks(d, known, docsRoot)
		linkProblems = append(linkProblems, probs...)
		gd = append(gd, graph.Doc{
			RelPath: d.RelPath,
			Dir:     d.Dir,
			Base:    d.Base,
			IsIndex: d.IsIndex,
			Links:   links,
		})
	}
	g := graph.Build(gd)

	out := pipeline.NewResult[document.Document](docs.Space)
	for _, d := range docs.Docs {
		nd, ok := g.Node(d.RelPath)
		if !ok {
			continue // 输入与图谱一致, 理论不可达.
		}
		out.Docs = append(out.Docs, document.Document{
			Document:  d,
			Parent:    nd.Parent,
			Children:  nd.Children,
			Links:     nd.Links,
			Referrers: nd.Referrers,
			Variants:  nd.Variants,
		})
	}
	pipeline.SortByKey(out.Docs)
	out.Problems = linkProblems
	return out, nil
}

// resolveLinks 从文档 AST 提取站内文档链接并解析为 RelPath.
//
// 返回去重后的链接列表, 以及断链等非致命问题.
func resolveLinks(d parser.Document, known map[string]bool, docsRoot string) (links []string, problems []pipeline.Problem) {
	seen := map[string]bool{}
	_ = ast.Walk(d.AST, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		l, ok := n.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}
		dest := string(l.Destination)
		targets, ok := resolveTarget(d, dest, docsRoot)
		if !ok {
			return ast.WalkContinue, nil // 外链/锚点/非文档, 不构成图边.
		}
		rel := ""
		for _, c := range targets {
			if known[c] {
				rel = c
				break
			}
		}
		if rel == "" {
			problems = append(problems, problem(pipeline.SeverityWarning, d.RelPath,
				"链接目标 %q 未指向已知文档", dest))
			return ast.WalkContinue, nil
		}
		if !seen[rel] {
			seen[rel] = true
			links = append(links, rel)
		}
		return ast.WalkContinue, nil
	})
	return links, problems
}

// resolveTarget 把链接目标解析为站内文档候选 RelPath 列表.
//
// docsRoot 是文文档根目录名 (跟随 Space.Layout.Docs, 默认 "docs"), 链接解析
// 必须与文档 RelPath 前缀一致. 返回 (nil, false) 表示非站内文档链接
// (外链 / 锚点 / 空 / 超出 docs 根), 不构成图谱边. 多个候选按优先级排列
// (原样、补 .md/.markdown、目录→入口文档), 由调用方在已知文档中选第一个命中项.
func resolveTarget(d parser.Document, dest, docsRoot string) ([]string, bool) {
	if dest == "" || strings.HasPrefix(dest, "#") {
		return nil, false
	}
	u, err := url.Parse(dest)
	if err != nil || u.IsAbs() || u.Host != "" {
		return nil, false // 外链或非法.
	}
	clean := dest
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	if clean == "" {
		return nil, false
	}

	docDir := docsRoot
	if d.Dir != "" {
		docDir = docsRoot + "/" + d.Dir
	}
	var res string
	if strings.HasPrefix(clean, "/") {
		res = strings.TrimLeft(clean, "/") // 站点内绝对路径: 相对 Space 根.
	} else {
		res = path.Clean(path.Join(docDir, strings.TrimPrefix(clean, "./")))
	}
	res = strings.TrimSuffix(res, "/")
	if !strings.HasPrefix(res, docsRoot+"/") {
		return nil, false // 站点内只有 docs 根下的 md 是文档.
	}

	cands := []string{res}
	switch strings.ToLower(path.Ext(res)) {
	case ".md", ".markdown":
		// 已是文档路径, 不再补扩展名.
	default:
		cands = append(cands, res+".md", res+".markdown")
		// 目录链接 (如 guide/) → 该目录的入口文档.
		cands = append(cands,
			res+"/index.md", res+"/index.markdown",
			res+"/_index.md", res+"/README.md")
	}
	return dedup(cands), true
}

// dedup 去重并保持顺序.
func dedup(items []string) []string {
	if len(items) < 2 {
		return items
	}
	seen := make(map[string]bool, len(items))
	out := items[:0]
	for _, it := range items {
		if seen[it] {
			continue
		}
		seen[it] = true
		out = append(out, it)
	}
	return out
}
