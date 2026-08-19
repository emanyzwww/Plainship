# core/build - 构建编排

## 描述

串联解析管线各阶段 (scan → parse → normalize), 汇总跨阶段问题, 产出最终构建结果.

## 产出

- `Docs`: 标准化后的文档列表 (共享脊柱 `pipeline.Doc` + parser 载荷).
- `Summary`: 跨阶段问题统计 (总数 / warning / error).
- `Problems` / `ProblemsByStage()`: 全部问题及按阶段分组, 供 CLI/UI 逐层展示.

后续将按 `pipeline.Stage` 接口接入 assembly / derive / render, 由本包统一编排.
