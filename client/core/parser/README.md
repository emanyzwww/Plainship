# core/parser - 解析层

## 描述

解析 Markdown 和 Front Matter, 提取元数据, 并进行标准化处理.

## 职责边界

parser 只回答 **"文件内容是什么"** —— 提取 Front Matter 元数据与 Markdown 的 goldmark AST, 不做任何语义解释:

| 问题                                  | 归属                   |
|---------------------------------------|------------------------|
| 有哪些文件 / 什么类型 / 什么路径      | core/scanner           |
| Front Matter 是什么 / 正文是什么      | core/parser            |
| 语言后缀 / 入口文档 / 标题兜底 / Slug | core/parser/normalizer |
| 文档之间的关系与站点图谱              | core/assembly          |
| 导航 / 分页 / 搜索等派生数据          | core/derive            |

parser 是纯读操作且幂等: 它不写任何文件, 可被安全重复调用; 对单个文件的异常不中断批次, 而是收集进 `Result.Problems` (与
scanner 的容错哲学一致).

## 输入与输出 (契约)

- 输入是 `*scanner.Result` (scanner 扫描产出的文档索引); 调用链为 `scan → parse → normalize`.
- 每个 `Document` 携带 `Entry` (来源条目)、`Meta` (Front Matter map)、`AST` (goldmark 语法树)、
  `Body` (去 FM 后的正文) 与 `Hash` (原始内容 SHA-256, 供未来增量/缓存比对).
- `Result.Problems` 直接复用 `scanner.Problem` 类型: 从扫描到解析再到后续各层, 全管线共享同一种问题形态, 便于逐层汇总与展示.
- normalizer 输出独立契约 `normalizer.Document` (含推导出的 `Base` / `Lang` / `IsIndex` / `Title` / `Slug`), 供 assembly
  直接消费.

## 用法

```go
sp := &space.Space{Root: "/path/to/site"}
scanned, err := scanner.Scan(sp)        // 第一步: 扫描
parsed, err := parser.Parse(scanned)    // 第二步: 解析 (可加 ParseWithOptions)
normalized, err := normalizer.Normalize(parsed) // 第二步收尾: 标准化
```

## Front Matter 规则

- 仅当文件第一行是 `---` 时视为存在 Front Matter; 随后第一个 `---` 行为闭合行.
- 正文中再出现的 `---` 行不影响切分 (属于正文, 渲染为 Markdown 分隔线).
- 解析失败 (坏 YAML / 内容不是映射) → error 级 Problem, 该文档按无元数据处理, 不中断批次.
- 分隔行存在但未闭合 → warning 级 Problem, 正文回退为整篇原文.
- UTF-8 BOM 在解析前剥离, 但 `Hash` 始终基于原始文件内容计算.

## 约定与扩展点

本层按 "只加不改" 原则设计, 未来功能应落在以下已预留的缝上, 而不是改已有语义:

1. **goldmark 实例集中初始化**: 所有解析共用 `newMarkdown()` 返回的同一实例, 新增 Markdown 能力 (GFM
   表格/脚注/代码高亮等) 只需在此处注册扩展, 层内其它代码零改动. 不要在各处各自 `goldmark.New()`.
2. **元数据保持 map**: Front Matter 按 `map[string]any` 原样保留, 键名含义归消费方 (normalizer / assembly / derive)
   按需读取. 新增 `draft` / `weight` / `aliases` 等键, 无需改动 parser 任何代码.
3. **契约类型只加字段, 不改已有字段语义**: `Document` 及 normalizer 的 `Document` 新增字段 (摘要/目录/草稿标记等) 直接追加,
   `Entry` / `Meta` / `AST` / `Body` / `Hash` 语义不变.
4. **Problems 复用 scanner.Problem**: 新增校验/告警逻辑只需追加 Problem, 永不改变已有条目形态.
5. **下游禁止摸本层内部**: assembly/render 只能消费 `Document` 契约, 不得依赖 parser 的 私有函数; 这样 parser 内部实现可自由演进.
6. **语义解释交给 normalizer**: parser 只提取不解释, 语言后缀/入口文档/标题兜底等 全部集中在 normalizer, 新增语义识别只改
   normalizer, 不影响 parser.

## 测试

`go test ./core/parser/...` : frontmatter (切分/解码边界), markdown (AST 结构), parse (整合/容错/幂等), normalizer
(语言/索引/标题/slug).