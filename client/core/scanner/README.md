# core/scanner - 扫描层

## 描述

扫描整个 PaperShip Space, 识别文件类型, 建立文件索引, 为后续处理做准备.

## 职责边界

scanner 只回答 **"有哪些文件, 是什么类型, 在哪"**, 不做内容级工作:

| 问题                             | 归属          |
|----------------------------------|---------------|
| 有哪些文件 / 什么类型 / 什么路径 | scanner       |
| 文件内容 (mdast / Front Matter)  | core/parser   |
| 文档之间的关系与站点图谱         | core/assembly |
| 导航 / 分页 / 搜索等派生数据     | core/compiler |

scanner 是纯读操作且幂等: 它不写任何文件, 可被安全重复调用; 对单个文件的异常不中断扫描, 而是收集进 `Result.Problems`.

## 与 model/space 的配合

- 输入是 `*space.Space`: 调用方负责确认目录是 Space 根 (`space.IsSpaceRoot`), 传入带 `Root` 的 Space.
- `Layout` 若为零值, 扫描时自动回填为标准布局 (`space.DefaultLayout`), 并写回 Space.
- 扫描时探测 `GitRoot` / `GitAvailable` 并回填到 Space.
- 配置文件 (`papership.yaml` / `.papership/config.yaml`) 只做 存在性检测并记录到 Result; **YAML 解析不属于 scanner**,
  留给配置加载层接入 yaml 库后处理.

## 产出

`Result` 包含:

- `Docs`: 文档索引, 供 parser 与 graph 消费.
- `Space`: 扩展后的 Space (已回填 Layout 与 Git 信息).
- `Assets`: 静态资源.
- `Themes`: themes 目录下的一级主题清单.
- `Problems`: 扫描中收集的警告与错误.

## 用法

```go
sp := &space.Space{Root: "/path/to/site"}
res, err := scanner.Scan(sp)
if err != nil { /* 根级错误 */ }
for _, d := range res.Docs { /* 交给 parser */ }

// 自定义选项: 包含点文件, 跳过主题收集.
res2, _ := scanner.ScanWithOptions(sp, scanner.ScanOptions{
IncludeDotFiles: true,
SkipThemes:      true,
})
```

## 约定与扩展点

- 文件类型: `.md` / `.markdown` 为文档, 其余为静态资源.
- DocEntry.Stem 只剥离扩展名; 语言后缀 (`intro.zh.md`)、入口文档 (`index` / `_index` / `README`) 等语义识别不属于
  scanner, 由 downstream (normalizer / assembly) 处理.
- 主题以 themes 下的一级目录为准; 散落的说明文件不收集.
- `Kind` 枚举目前是"预留"类型, 尚未挂到任何 Entry 上; 待后续需要统一的文件清单时再接线.
- 跳过规则: `build` / `state` / `themes` 子树与 `.git`, 默认也跳过点文件和目录 (`IncludeDotFiles` 可开启).
- 可注入替身 (`osStat` / `walkDir`) 与 model/space 的注入模式一致, 便于单元测试.
- 增量扫描: DocEntry/AssetEntry 已携带 `ModTime` 与 `Size`, 后续可基于 StateDir 实现增量与 watch.
