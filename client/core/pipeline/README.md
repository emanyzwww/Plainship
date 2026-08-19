# core/pipeline - 管线共享契约

## 描述

全管线 (scanner → parser → normalizer → assembly → derive → render → output) 共享的 **基础设施与稳定契约**所在的中性包.
各层只挂自己的载荷, 不重复声明公共结构.

## 共享内容

| 类型 / 函数                                    | 说明                                                    |
|------------------------------------------------|---------------------------------------------------------|
| `Problem` / `Severity`                         | 全管线统一的问题形态; 带 `Stage` 来源标记, 可跨阶段汇总 |
| `Doc`                                          | 文档脊柱: 身份 + 语义字段, 按生命周期由各层填充         |
| `Result[T]`                                    | 通用结果信封: Space 透传 + Docs + Problems + 计数方法   |
| `SortByKey`                                    | 一份排序实现, 全管线复用 (内嵌 `Doc` 的类型天然满足)    |
| `Summarize` / `GroupByStage` / `MergeProblems` | 跨阶段问题统计与分组                                    |
| `Stage[In,Out]` / `FuncStage`                  | 阶段接口, 供编排层统一串联                              |

## 归属边界

- **共享**: 问题形态、结果信封、文档脊柱、排序/汇总实现、阶段契约.
- **局部 (留在各层)**: parser 的 `Meta/AST/Body`, assembly 的图关系, render 的 HTML 等载荷; 以及 scanner 的文件系统视图
  `DocEntry/AssetEntry` (含 AbsPath, 与脊柱语义视图不同).

## 生命周期

脊柱 `Doc` 的字段按管线顺序逐步填充:

```
scanner    → 物理事实: RelPath/Dir/Stem/Ext/Size/ModTime
parser     → 内容事实: Hash (+ 载荷 Meta/AST/Body)
normalizer → 语义事实: Base/Lang/IsIndex/Title/Slug
```

下游通过内嵌 `Doc` 获得统一字段, 新增语义只改 normalizer 一处.
