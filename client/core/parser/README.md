# core/parser - 解析层

## 定位

解析 Markdown 与 Front Matter, 提取元数据, 并完成标准化 (含子层 normalizer).

parser 只回答 **"文件内容是什么"** — 提取 Front Matter 元数据与 Markdown 的 goldmark AST, 不做任何语义解释.

## 管线位置

scanner → **parser**(本层) → **normalizer**(本层收尾) → assembly → derive → render → output → distribution

## 职责边界

| 问题                                  | 归属                   |
|---------------------------------------|------------------------|
| 有哪些文件 / 什么类型 / 什么路径      | core/scanner           |
| Front Matter 是什么 / 正文是什么      | **core/parser**        |
| 语言后缀 / 入口文档 / 标题兜底 / Slug | core/parser/normalizer |
| 文档之间的关系与站点图谱              | core/assembly          |
| 导航 / 分页 / 搜索等派生数据          | core/derive            |

parser 是纯读操作且幂等: 不写任何文件, 可安全重复调用; 单个文件的异常不中断批次, 收集进 `Problems` (与 scanner
的容错哲学一致).

## 输入与输出

- 输入: `*scanner.Result` (scanner 产出的文档索引).
- 输出: `*pipeline.Result[Document]` — 每个 `Document` 内嵌共享脊柱 `pipeline.Doc` (物理/内容事实), 携带
  `Meta` (Front Matter map)、`AST` (goldmark 语法树) 与 `Body` (去 FM 后的正文); 语义字段 (`Base`/`Lang`/
  `IsIndex`/`Title`/`Slug`) 由 normalizer 推导后写回脊柱.
- 调用链: `scan → parse(本层) → normalize(本层收尾)`; normalizer 输出直接供 assembly 消费.

## 用法

```go
sp := &space.Space{Root: "/path/to/site"}
scanned, err := scanner.Scan(sp) // 第一步: 扫描
parsed, err := parser.Parse(scanned) // 第二步: 解析 (可加 ParseWithOptions)
normalized, err := normalizer.Normalize(ctx, parsed) // 第二步收尾: 标准化

// 或直接用编排入口, 自动汇总各阶段问题
res, err := build.Run(ctx, sp)
```

## 约定与扩展点

本层按 "只加不改" 原则设计, 未来功能应落在已预留的缝上, 而不是改已有语义:

1. **goldmark 实例集中初始化**: 解析与渲染共用 `core/markdown` 提供的同一实例, 新增 Markdown 能力 (GFM 表格/脚注/
   代码高亮等) 只需在 `core/markdown` 注册扩展, parser/render 零改动. 不要在各处各自 `goldmark.New()`.
2. **元数据保持 map**: Front Matter 按 `map[string]any` 原样保留, 键名含义归消费方 (normalizer / assembly / derive)
   按需读取. 新增 `draft` / `weight` / `aliases` 等键, 无需改动 parser 代码.
3. **契约只加不改**: 脊柱 `pipeline.Doc` 与 `Document` 新增字段直接追加, 已有字段语义不变.
4. **问题形态共享**: `Problems` 复用 `pipeline.Problem`, 带 `Stage:"parser"` 来源标记, 永不改变已有条目形态.
5. **下游禁止摸本层内部**: assembly/render 只能消费 `Document` 契约, 不得依赖 parser 私有函数, 以便内部实现自由演进.
6. **语义解释交给 normalizer**: parser 只提取不解释, 语言后缀/入口文档/标题兜底等集中在 normalizer, 新增语义识别 只改
   normalizer, 不影响 parser.
7. **Front Matter 切分规则**: 仅第一行 `---` 视为存在 FM, 随后第一个 `---` 行为闭合行; 正文中再出现的 `---`
   属正文. 坏 YAML / 非映射 → error 级 Problem, 按无元数据处理不中断; 未闭合 → warning 级, 正文回退为整篇原文.
8. **BOM 处理**: UTF-8 BOM 解析前剥离, 但 `Hash` 始终基于原始文件内容计算.

## 测试

`go test ./core/parser/...` : frontmatter (切分/解码边界), markdown (AST 结构), parse (整合/容错/幂等), normalizer
(语言/索引/标题/slug).
