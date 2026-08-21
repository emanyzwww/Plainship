# core/build - 构建编排

## 定位

串联解析管线各阶段 (scan → parse → normalize → assemble → derive → render → output), 汇总跨阶段问题, 产出统一构建结果.

它位于管线顶端而不占阶段, 是 CLI 调用的入口.

## 管线位置

下列阶段由本包统一编排; distribution 不并入 Run (构建 ≠ 发布), 由 CLI 按命令独立触发:

scanner → parser → normalizer → assembly → derive → render → output

## 职责边界

| 问题                          | 归属                    |
|-------------------------------|-------------------------|
| 各阶段的业务内容              | 各层 (scanner/parser/…) |
| 编排阶段顺序 / 汇总跨阶段问题 | **core/build**          |

## 输入与输出

- 输入: `*space.Space`.
- 输出: `*build.Result` — `Docs` (标准化文档列表) + `Summary` (问题统计) + `Problems` / `ProblemsByStage()`
  (全部问题与按阶段分组, 供 CLI/UI 逐层展示).
- 顶层 `error` 仅代表管线无法继续; 单文件级问题进入 `Problems`, 不中断构建.

## 用法

```go
res, err := build.Run(ctx, &space.Space{Root: "/path/to/site"})
if err != nil { /* 根级错误 */ }
fmt.Printf("docs=%d warnings=%d errors=%d\n", res.DocCount(), res.Summary.Warnings, res.Summary.Errors)
```

## 约定与扩展点

1. **逐层接入**: 新阶段实现 `pipeline.Stage[In,Out]` 后在 `Run` 中排队, 并把其 `Problems` 并入汇总.
2. **汇总即展示源**: `ProblemsByStage()` 是给 CLI/UI 的唯一数据源, 界面不直接摸各层.

## 测试

`go test ./core/build/...` : 全链路编排 (scan→parse→normalize), 语义字段写入脊柱, 跨阶段问题汇总与分组.
