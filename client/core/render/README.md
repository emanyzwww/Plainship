# core/render - 渲染层

## 定位

用主题模板把派生页渲染成完整 HTML, 生成最终页面内容, 供输出层写盘.

布局来源经 fs.FS 注入 (nil 用本机 `os.DirFS`), 测试可用 `fstest.MapFS`; 主题或布局缺失时回退内置默认布局, 保证任何 Space
无需主题都能出站.

## 管线位置

scanner → parser → normalizer → assembly → derive → **render**(本层) → output → distribution

## 职责边界

| 问题                           | 归属            |
|--------------------------------|-----------------|
| 导航 / 分页 / 搜索等派生数据   | core/derive     |
| Markdown → HTML + 主题模板填充 | **core/render** |
| 把结果写入输出目录             | core/output     |
| 静态资源原样拷贝               | core/output     |

## 输入与输出

- 输入: `*derive.Result` (派生页, 含 URL/面包屑/上下一篇).
- 输出: `*render.Result` — 每页 `Page` 携带 `HTML` (完整页面) 与 `OutPath` (相对 BuildDir 的写盘路径);
  `Theme` 记录实际使用主题.
- Markdown 渲染: 经 `core/markdown` 共享引擎 (AST → HTML), 与 parser 的解析配置一致.

## 用法

```go
rendered, err := render.Render(ctx, derived) // derived 来自 derive.Derive; 无主题走内置布局
// 或自定义主题与虚拟文件系统:
res2, err := render.RenderWithOptions(ctx, derived, render.Options{
	Theme: "fancy",
	FS:    fstest.MapFS{"themes/fancy/layouts/page.html": {Data: []byte(layout)}},
})
for _, p := range rendered.Docs {
	fmt.Printf("%s -> %s (%dB)\n", p.URL, p.OutPath, len(p.HTML))
}
```

## 主题布局约定

- 布局目录: `<themes>/<theme>/layouts/`.
- 文件名即种类: `index.html` (根入口) / `section.html` (段入口) / `page.html` (普通页); 找不到具体种类时回退
  `_default.html`.
- 模板数据模型为 `ViewModel`: `SiteTitle` / `Title` / `URL` / `Content` (Markdown 渲染结果, 已信任, 不会二次转义) /
  `Breadcrumb` / `Prev` / `Next`.

## 约定与扩展点

1. **内置兜底布局**: 主题缺失 → warning 级 Problem + 内置布局; 布局加载/解析/执行失败 → error 级 Problem, 该页跳过不中断构建
   (与全管线容错哲学一致).
2. **`template.HTML` 防二次转义**: Markdown 渲染结果以 `template.HTML` 注入, go html/template 不再转义.
3. **视图模型隔离**: 模板只见 `ViewModel`, 不暴露内部结构, 便于未来加字段不影响模板.
4. **静态资源不归本层**: 主题 static 目录由 output 层原样拷贝, 本层只产出 HTML.
5. **扩展缝**: 新增模板函数 (本地化/日期格式化等) 未来在 `RenderWithOptions` 内注册 FuncMap.

## 测试

`go test ./core/render/...` : 主题布局执行 (正文不二次转义/面包屑/上下一篇), 种类布局 (index/section/page),
_default 兜底, 主题缺失回退内置布局, 坏模板容错, OutPath (含语言前缀), nil 输入, Stage 冒烟.
