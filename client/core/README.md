# core - 客户端核心总纲

客户端核心是把一个 PaperShip Space 构建并分发成站点的一整条流水线. 本目录按 **层**组织, 每层一个目录, 一条从扫描到分发的数据流.

## 管线总图

```
scanner → parser → normalizer → assembly → derive → render → output → distribution
  扫描        解析       标准化      组装      派生      渲染     输出      分发
```

- **阶段层** (参与管线): scanner / parser (+normalizer) / assembly / derive / render / output / distribution.
- **底座层** (非阶段, 被各层引用): markdown (共享渲染引擎), pipeline (共享契约), build (编排入口).

## 目录布局约定

- 每层一个目录, **包名 = 目录名**; 层的子能力放子目录 (如 `parser/normalizer`, `assembly/document`,
  `assembly/graph`), 子目录不参与管线位置.
- 数据与问题按 `pipeline` 的合同流动: 脊柱 `Doc` 逐层填充, 问题统一为 `Problem` 并带 `Stage` 来源标记, 各层输出
  `pipeline.Result[T]`.

```
client/core/
├── README.md            # 本总纲
├── scanner/             # 阶段: 扫描
├── parser/              # 阶段: 解析 (含子层 normalizer 标准化)
├── assembly/            # 阶段: 组装 (document / graph)
├── derive/              # 阶段: 派生
├── render/              # 阶段: 渲染
├── output/              # 阶段: 输出
├── distribution/        # 阶段: 分发
├── markdown/            # 底座: goldmark 共享引擎 (解析+渲染)
├── pipeline/            # 底座: 共享契约 (Problem/Doc/Result/Stage)
└── build/               # 底座: 构建编排
```

## README 模板契约

每个模块的 README 一律按以下 6 个固定章节、固定顺序、固定标题书写, 不增删、不换名、不调序:

1. `## 定位` — 一句话说明本层回答什么业务问题.
2. `## 管线位置` — 完整管线箭头, 本层加粗; 底座层说明其非阶段属性.
3. `## 职责边界` — 表格: 业务问题 → 归属层, 明确"什么归我、什么不归".
4. `## 输入与输出`— 输入类型 / 输出类型 / 调用链 (或该层承载的数据形态).
5. `## 用法` — 最小可用 `go` 代码示例.
6. `## 约定与扩展点` — 编号列表: 设计规则与预留扩展缝 ("只加不改" 原则在此沉淀).
7. `## 测试` — `go test` 命令 + 覆盖范围.

未落地层保持同一骨架, 未实现章节统一使用 `> ⏳ 待实现` 占位, 实现时原地填充, 不另造章节.
