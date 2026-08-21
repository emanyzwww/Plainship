# core/derive - 功能派生层

## 定位

基于站点图谱, 为每个页面与整站派生展示所需数据: 导航、树/面包屑/上下一篇、URL、站点地图、搜索索引.

本层是确定性只读转换: 不读文件、不写文件, 也不产生问题 (URL 冲突等约束见"约定与扩展点").

## 管线位置

scanner → parser → normalizer → assembly → **derive**(本层) → render → output → distribution

## 职责边界

| 问题                             | 归属            |
|----------------------------------|-----------------|
| 文档之间的关系与站点图谱         | core/assembly   |
| 导航树 / 面包屑 / 上下一篇 / URL | **core/derive** |
| 分类 / 分页 (预留)               | **core/derive** |
| 站点地图 / 搜索索引              | **core/derive** |
| 最终的 HTML 页面                 | core/render     |

## 输入与输出

- 输入: `*pipeline.Result[document.Document]` (assembly 产出的统一文档模型, 含图谱投影).
- 输出: `*derive.Result` — 每页 `Page` 携带 `URL` / `Nav` (面包屑) / `Section` / `Prev` / `Next`; 全局携带 `Nav`
  (导航树) / `SiteMap` (全部 URL) / `SearchIndex`.
- **URL 写回共享脊柱** `pipeline.Doc.URL`: render/output/sitemap 都从此取, 各层不再自行推导.

## 用法

```go
derived, err := derive.Derive(ctx, assembled) // assembled 来自 assembly.Assemble
if err != nil { /* 只有整层无法继续才返回 */ }
for _, p := range derived.Docs {
fmt.Printf("%s -> %s (section=%s)\n", p.RelPath, p.URL, p.Section)
}
```

## 约定与扩展点

1. **URL 规则 (clean URL)**: 去掉文档根前缀与扩展名, 路径段用 Base/Slug (剥离语言后缀); 入口文档 → 目录 URL
   (`docs/index.md` → `/`, `docs/guide/README.md` → `/guide/`); 文档根跟随 `Space.Layout.Docs`.
2. **语言变体 URL**: 同基名多语言互为变体 (由 assembly 标记); 组内默认语言不带前缀, 其余变体带
   `/<lang>/` 前缀. 默认语言规则: **lang 为空的变体恒为默认** → 站点默认语言 (`Config.SiteLanguage`)
   命中 → RelPath 最小者. **同基名多语言必须互为变体, 否则 URL 冲突**.
3. **导航顺序**: 节点与子节点按 RelPath 字典序 (确定性); 未来按 Front Matter `weight` 排序的扩展缝 已保留 (Meta 随文档前传,
   derive 可读取).
4. **分页 / 分类预留**: 本层是它们的自然归属, 后续在 Result 上追加派生集合即可, 不改变已有字段.
5. **本层无问题产出**: 派生规则均为确定性转换; 校验类问题留给未来校验层或 CLI 汇总展示层.
6. **Slug 兜底**: URL 路径段优先 `Slug`, 为空时回退 `Base` (正常由 normalizer 填充).

## 测试

`go test ./core/derive/...` : URL 派生 (入口/普通/自定义文档根/语言前缀), 导航树与面包屑, 上下一篇, 站点地图与搜索索引
(纯文本提取), 纤维层 nil 输入, Stage 接口冒烟.
