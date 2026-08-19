package pipeline

import "sort"

// Keyed 是可按文档键排序的类型; pipeline.Doc 及内嵌它的类型通过 Key() 满足.
type Keyed interface{ Key() string }

// Key 返回文档排序键: 相对 Space 根目录的路径, 与各层排序约定一致.
func (d Doc) Key() string { return d.RelPath }

// SortByKey 按 Key() 升序排序, 一份实现供全管线复用.
func SortByKey[T Keyed](docs []T) {
	sort.Slice(docs, func(i, j int) bool { return docs[i].Key() < docs[j].Key() })
}
