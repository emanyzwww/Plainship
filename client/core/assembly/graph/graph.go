// Package graph 构建站点图谱: 从文档列表建立目录层级、内部链接与语言变体关系.
//
// 本包只依赖文档的最小事实 (graph.Doc), 不感知 parser/document 结构,
// 由上层 (core/assembly) 负责把解析结果投影为 graph.Doc 并消费出图.
package graph

import (
	"path"
	"sort"
	"strings"
)

// Doc 是建图所需的文档最小事实.
type Doc struct {
	RelPath string   // RelPath 相对 Space 根目录的路径, 如 "docs/guide/intro.md".
	Dir     string   // Dir 相对 docs 根目录的目录部分; 顶层文档为空.
	Base    string   // Base 剥离语言后缀后的基名.
	IsIndex bool     // IsIndex 是否为入口文档 (index/_index/README).
	Links   []string // Links 本文档出向内部链接 (已解析为 RelPath, 去重).
}

// Node 是图谱中一个文档节点的投影信息.
type Node struct {
	RelPath   string   // RelPath 节点路径.
	Dir       string   // Dir 节点所在目录 (相对 docs 根).
	Base      string   // Base 剥离语言后缀后的基名.
	IsIndex   bool     // IsIndex 是否为入口文档.
	Parent    string   // Parent 父节点 RelPath (目录层级); 顶层为空.
	Children  []string // Children 直接子节点 RelPath, 按 RelPath 升序.
	Links     []string // Links 出向内部链接 (已解析为 RelPath, 去重).
	Referrers []string // Referrers 引用本文档的节点 RelPath (反向边).
	Variants  []string // Variants 同 Base 的语言变体 RelPath (含自身); 无变体时为空.
}

// Graph 是一次构建的站点图谱.
type Graph struct {
	nodes map[string]*Node // nodes key 为 RelPath.
	order []string         // order 全部节点 RelPath, 按 RelPath 升序.
}

// Build 从文档最小事实构建完整图谱 (层级 + 链接边 + 语言变体).
//
// 输入无序亦可, 内部统一按 RelPath 排序, 保证结果确定性.
func Build(docs []Doc) *Graph {
	g := &Graph{nodes: make(map[string]*Node, len(docs))}
	for _, d := range docs {
		g.nodes[d.RelPath] = &Node{
			RelPath: d.RelPath,
			Dir:     d.Dir,
			Base:    d.Base,
			IsIndex: d.IsIndex,
			Links:   dedupSort(d.Links),
		}
		g.order = append(g.order, d.RelPath)
	}
	sort.Strings(g.order)

	g.buildHierarchy()
	g.buildReferrers()
	g.buildVariants()
	return g
}

// Node 返回指定文档的节点; 不存在时 ok 为 false.
func (g *Graph) Node(rel string) (*Node, bool) {
	n, ok := g.nodes[rel]
	return n, ok
}

// Order 返回全部节点 RelPath, 按 RelPath 升序.
func (g *Graph) Order() []string { return g.order }

// Len 返回节点数量.
func (g *Graph) Len() int { return len(g.nodes) }

// buildHierarchy 建立目录层级: 每个目录的入口文档是该目录的"段节点",
// 节点的父节点 = 其所在目录的入口文档, 否则逐级上溯到最近的入口文档.
func (g *Graph) buildHierarchy() {
	// dirIndex: 每个目录的入口文档 (IsIndex 且 Dir 为该目录).
	dirIndex := map[string]string{}
	for _, rel := range g.order {
		n := g.nodes[rel]
		if n.IsIndex {
			if _, exists := dirIndex[n.Dir]; !exists {
				dirIndex[n.Dir] = rel // 同目录多个入口时取 RelPath 最小的.
			}
		}
	}

	for _, rel := range g.order {
		n := g.nodes[rel]
		dir := n.Dir
		for {
			if p, ok := dirIndex[dir]; ok && p != rel {
				n.Parent = p
				break
			}
			if dir == "" {
				break
			}
			dir = path.Dir(dir)
			if dir == "." {
				dir = ""
			}
		}
	}

	for _, rel := range g.order {
		n := g.nodes[rel]
		if n.Parent != "" {
			if p := g.nodes[n.Parent]; p != nil {
				p.Children = append(p.Children, rel)
			}
		}
	}
}

// buildReferrers 计算反向边: 哪些文档引用了每个节点.
func (g *Graph) buildReferrers() {
	for _, rel := range g.order {
		n := g.nodes[rel]
		for _, target := range n.Links {
			if t, ok := g.nodes[target]; ok && target != rel {
				t.Referrers = append(t.Referrers, rel)
			}
		}
	}
}

// buildVariants 按 "目录/基名" 分组, 把同基名的语言变体互相标记.
func (g *Graph) buildVariants() {
	byKey := map[string][]string{}
	for _, rel := range g.order {
		n := g.nodes[rel]
		key := strings.TrimSuffix(n.Dir+"/", "/") + "/" + n.Base
		byKey[key] = append(byKey[key], rel)
	}
	for _, rels := range byKey {
		if len(rels) < 2 {
			continue
		}
		for _, r := range rels {
			g.nodes[r].Variants = rels // 含自身: 便于按任意变体找"其它语言 URL".
		}
	}
}

// dedupSort 去重并按 RelPath 升序.
func dedupSort(items []string) []string {
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
	sort.Strings(out)
	return out
}
