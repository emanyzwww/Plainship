# core/assembly - 组装层

## 定位

构建统一的文档模型, 并基于全部文档模型构建站点图谱, 建立文档之间的各种关系.

本层出现后, 脊柱 `pipeline.Doc` 上会挂上本层独有的载荷 (图 / 关系).

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

本层按 `pipeline.Stage` 接口接入, 编排由 core/build 统一串联.

## 输入与输出

> ⏳ 待实现: 该层尚未落地, 输入类型 / 输出契约 / 调用链在实现时补充.

## 用法

> ⏳ 待实现: 该层尚未落地, 用法示例随实现补充.

## 约定与扩展点

> ⏳ 待实现: 该层尚未落地, 设计规则与扩展缝在实现时沉淀.

## 测试

> ⏳ 待实现: 该层尚未落地, 测试命令与覆盖说明在实现时补充.

