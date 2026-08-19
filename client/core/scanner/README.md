# core/scanner - 扫描层

## 定位

识别 Space 下"有哪些文件、是什么类型、在哪", 建立文件索引, 为后续处理做准备.

scanner 只回答 **"有哪些文件, 是什么类型, 在哪"**, 不做内容级工作.

## 管线位置

**scanner**(本层) → parser → normalizer → assembly → derive → render → output → distribution

## 职责边界

| 问题                             | 归属          |
|----------------------------------|---------------|
| 有哪些文件 / 什么类型 / 什么路径 | **scanner**   |
| 文件内容 (mdast / Front Matter)  | core/parser   |
| 文档之间的关系与站点图谱         | core/assembly |
| 导航 / 分页 / 搜索等派生数据     | core/derive   |

scanner 是纯读操作且幂等: 不写任何文件, 可安全重复调用; 单个文件的异常不中断扫描, 收集进 `Problems`.

## 输入与输出

- 输入: `*space.Space` (调用方负责确认目录是 Space 根, 传入带 `Root` 的 Space).
- 产出: `*scanner.Result` — `Docs` / `Themes` / `Assets` / `Problems`, 并把探测到的 `Layout` /
  `GitRoot` / `GitAvailable` 回填到 Space.
- 调用链: `scan` (本层产出直接供 parser 消费).

## 用法

```go
sp := &space.Space{Root: "/path/to/site"}
res, err := scanner.Scan(sp)
if err != nil { /* 只有 Space 根级错误才返回 */ }
for _, d := range res.Docs { /* 交给 parser */ }

// 自定义选项: 包含点文件, 跳过主题收集.
res2, _ := scanner.ScanWithOptions(sp, scanner.ScanOptions{
IncludeDotFiles: true,
SkipThemes:      true,
})
```

## 约定与扩展点

1. **文件类型**: `.md` / `.markdown` 为文档, 其余为静态资源; 文档只在 docs 目录下, 根目录散落的 md 属于静态资源.
2. **语义识别归属下游**: `DocEntry.Stem` 只剥离扩展名; 语言后缀 (`intro.zh.md`)、入口文档 (`index` /
   `_index` / `README`) 等语义识别由 normalizer / assembly 处理.
3. **主题收集规则**: 以 themes 下的一级目录为准, 散落的说明文件不收集.
4. **`Kind` 枚举预留**: 目前是"预留"类型, 尚未挂到任何 Entry 上, 待需要统一文件清单时再接线.
5. **跳过规则**: `build` / `state` / `themes` 子树与 `.git`, 默认也跳过点文件和目录 (`IncludeDotFiles` 可开启).
6. **问题形态共享**: `Problem` / `Severity` 复用 `pipeline` 全管线统一形态, 带 `Stage:"scanner"` 来源标记.
7. **可注入替身**: `osStat` / `walkDir` 与 model/space 的注入模式一致, 便于单元测试.
8. **增量扫描预留**: `DocEntry` / `AssetEntry` 已携带 `ModTime` 与 `Size`, 后续可基于 StateDir 实现增量与 watch.

## 测试

`go test ./core/scanner/...` : 文档/资源/主题分类, 跳过规则与点文件, 问题分级与容错, Git 与布局探测, 幂等性.
