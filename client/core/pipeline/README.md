# core/pipeline - 管线共享底座

## 定位

全管线共享的 **基础设施与稳定契约**: 问题形态、结果信封、文档脊柱、排序与汇总实现、阶段接口. 各层只挂自己的载荷, 不再重复声明公共结构.

它不是一个管线阶段, 而是横贯全部阶段的共享底座.

## 管线位置

横贯所有阶段 (scanner → parser → normalizer → assembly → derive → render → output), 供各阶段 import.

## 职责边界

| 内容                                        | 归属         |
|---------------------------------------------|--------------|
| `Problem` / `Severity`                      | **pipeline** |
| `Doc` (文档脊柱)                            | **pipeline** |
| `Result[T]` / 排序 / 汇总                   | **pipeline** |
| `Stage[In,Out]` 阶段契约                    | **pipeline** |
| parser 的 `Meta/AST/Body`                   | core/parser  |
| scanner 的文件视图 `DocEntry`/ `AssetEntry` | core/scanner |
| assembly 的图关系 / render 的 HTML          | 各自层内     |

## 输入与输出

- 数据形态 (依次流经所有阶段): `Problem` → `Doc` (脊柱) → `Result[T]` (信封).
- 脊柱 `Doc` 字段按生命周期逐步填充:
  `scanner`(物理事实: RelPath/Dir/Stem/Ext/Size/ModTime) → `parser`(内容事实: Hash) → `normalizer`(语义事实:
  Base/Lang/IsIndex/Title/Slug).

## 用法

```go
// 下游层内嵌脊柱, 天然获得统一字段与排序键.
type Document struct {
pipeline.Doc // 身份 + 语义字段
Meta map[string]any       // 本层载荷
AST  *ast.Document
Body []byte
}

// 信封直接用 pipeline 的.
res := pipeline.NewResult[Document](sp)
pipeline.SortByKey(res.Docs)
```

## 约定与扩展点

1. **只加不改**: 脊柱与 `Problem` 新增字段直接追加, 已有字段语义不变.
2. **载荷留在各层**: 脊柱放全管线公认的事实, 各层私有载荷 (AST/HTML/图) 留在层内, 避免层间过度耦合.
3. **新增语义只改一处**: 语言/入口/标题/slug 等语义集中在 normalizer.
4. **问题统一汇总**: 各层问题带 `Stage` 来源标记, 供 build 汇总与 CLI/UI 分组展示.

## 测试

`go test ./core/pipeline/...` : 结果信封, 通用排序, 问题构造/统计/分组, 函数式 Stage 适配.
