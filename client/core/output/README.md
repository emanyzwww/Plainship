# core/output - 输出层

## 定位

把渲染结果写入输出目录, 生成完整静态站点与附加文件: 页面 HTML、静态资源、sitemap/search-index/robots.

逐文件失败收集为 Problem 并继续出盘, 不中断整站 (与全管线容错哲学一致).

## 管线位置

scanner → parser → normalizer → assembly → derive → render → **output**(本层) → distribution

## 职责边界

| 问题                           | 归属              |
|--------------------------------|-------------------|
| Markdown → HTML + 主题模板填充 | core/render       |
| 页面 / 静态资源 / 附加文件落盘 | **core/output**   |
| 打包并推送到服务端             | core/distribution |

## 输入与输出

- 输入: `*output.Input` — 各层产物的汇合点: `Space` / `Theme` (来自 render) / `Pages` (HTML+OutPath) /
  `Assets` (来自 scan) / `SiteMap` + `Search` (来自 derive).
- 输出: `*output.Result` — `Docs` 为写盘清单 (`Written{Path, Bytes}`, 相对 BuildDir).
- 落盘规则:
    - 页面: <build>/<OutPath> (clean URL, 如 `guide/intro/index.html`);
    - 静态资源: docs 下非文档文件剥掉 docs 前缀 (`docs/img/logo.png` → `img/logo.png`), 根级资源原样;
    - 主题 static: `themes/<theme>/static/*` 原样拷贝到 build 根; 无 static 目录静默跳过;
    - 附加文件: `sitemap.xml` (SiteURL + 页面 URL) / `search-index.json` / `robots.txt` (SiteURL 非空时含 Sitemap 行).

## 用法

```go
written, err := output.Write(ctx, &output.Input{
	Space:   sp,
	Theme:   rendered.Theme,
	Pages:   rendered.Docs,
	Assets:  scanned.Assets,
	SiteMap: derived.SiteMap,
	Search:  derived.SearchIndex,
})
if err != nil { /* 只有整层无法继续才返回 */ }
fmt.Printf("写出 %d 个文件\n", written.DocCount())
```

## 约定与扩展点

1. **只写新增, 不清理旧档**: 当前每次构建增量写盘; 清除陈旧文件 (clean build) 留待后续基于 StateDir 实现.
2. **sitemap 绝对地址**: `SiteURL` 为空时 loc 用相对路径 (URL 本身), 配置站点地址后自动变绝对地址.
3. **搜索索引即是后续 CLI 展示的数据源**: search-index.json 由 derive.SearchEntry 序列化 (带 json 标签).
4. **附加文件追加缝**: 新附件 (如 `feed.xml` / `404.html`) 在 `addExtras` 里追加, 不改变已有产出.

## 测试

`go test ./core/output/...` : 整站出盘 (页面/资源/主题 static/附加文件全断言), 自定义文档根, 资源缺失容错, nil 输入,
Stage 冒烟.
