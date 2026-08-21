# core/assembly - 组装层

## 定位

构建统一的文档模型, 并基于全部文档模型构建站点图谱, 建立文档之间的各种关系 (目录层级 / 内部链接 / 语言变体).

本层把图谱投影 (Parent/Children/Links/Referrers/Variants) 挂到每篇文档模型上, 供 derive/render 直接消费.

## 管线位置

scanner → parser → normalizer → **assembly**(本层) → derive → render → output → distribution

## 职责边界

| 问题                             | 归属              |
|----------------------------------|-------------------|
| 有哪些文件 / 什么类型 / 什么路径 | core/scanner      |
| Front Matter 是什么 / 正文是什么 | core/parser       |
| 文档之间的关系与站点图谱         | **core/assembly** |
| 导航 / 分页 / 搜索等派生数据     | core/derive       |
| 最终的 HTML 页面                 | core/render       |

内部链接的 **含义解析** (导航顺序 / 分页权重等) 不属于本层, 归 derive.

## 输入与输出

- 输入: `*pipeline.Result[parser.Document]` (normalizer 产出的标准化文档).
- 输出: `*pipeline.Result[document.Document]` — 每篇文档内嵌 `parser.Document` 全量事实, 并携带图谱投影:
  `Parent` / `Children` / `Links` / `Referrers` / `Variants`.
- 结构与调用链: `assembly/document` (统一文档模型) + `assembly/graph` (图谱构建), 由本包 `Assemble` 串联; 本层只报本阶段问题,
  跨阶段汇总由 core/build 负责.

## 用法

```go
assembled, err := assembly.Assemble(normalized) // normalized 来自 normalizer.Normalize
if err != nil { /* 只有整层无法继续才返回 */ }
for _, d := range assembled.Docs {
fmt.Printf("%s -> parent=%s links=%v\n", d.RelPath, d.Parent, d.Links)
}
```

## 约定与扩展点

1. **链接解析规则**: 站内相对路径 / 站点内绝对路径 → 解析为 `docs/...` 的 RelPath; 外链 (http/https/mailto 等绝对 URL)
   、页内锚点 (`#`) 不构成图边; `?fragment` 前的路径参与解析.
2. **省略扩展名兼容**: 链接可省略 `.md` / `.markdown`, 按 原样 → 补 `.md` → 补 `.markdown` 的顺序匹配已知文档.
3. **断链容错**: 指向未知文档的链接 → warning 级 Problem (`Stage:"assembly"`), 文档照常产出, 不中断构建.
4. **目录层级约定**: 每个目录的入口文档 (`index`/`_index`/`README`) 是该目录的段节点; 节点父级 = 所在目录的 段节点,
   否则逐级上溯; 同目录多个入口时取 RelPath 最小者.
5. **wiki 双链预留**: 目前只做标准链接; 未来支持 `[[双链]]` 需加 goldmark 扩展, 在 `resolveLinks` 缝上接.
6. **图谱只放关系**: `graph` 只依赖 `graph.Doc` 最小事实, 不感知 parser/document, 上层负责投影.

## 测试

`go test ./core/assembly/...` : 目录层级与子节点顺序, 链接解析 (相对/跨目录/外链/锚点/断链), 语言变体分组, 反向边,
输入无序确定性, nil 输入, 只报本阶段问题.
