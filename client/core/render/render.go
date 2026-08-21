// Package render 是渲染层入口: 用主题模板把派生页渲染成完整 HTML 页面.
//
// 输入 derive 的派生结果, 输出每页的 HTML + 输出相对路径 (供 output 写盘).
// 布局来源经 fs.FS 注入 (nil 用本机 os.DirFS), 测试可用 fstest.MapFS;
// 主题缺失或布局缺失时回退内置默认布局, 保证任何 Space 都能出站.
package render

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"strings"

	"github.com/emanyzwww/papership-client/core/derive"
	"github.com/emanyzwww/papership-client/core/markdown"
	"github.com/emanyzwww/papership-client/core/pipeline"
)

// Options 控制渲染行为; 零值即默认行为.
type Options struct {
	Theme string // Theme 主题名; 空 → Space.Config.ThemeName → "default".
	FS    fs.FS  // FS 主题布局来源 (以 Space 根为根的 fs.FS); nil → 本机 os.DirFS.
}

// Page 是渲染完成的页面.
type Page struct {
	derive.Page
	HTML    []byte // HTML 完整页面.
	OutPath string // OutPath 输出相对路径 (相对 BuildDir), 如 "guide/intro/index.html".
}

// Result 是一次 Render 的完整产物.
type Result struct {
	pipeline.Result[Page]        // Pages + Space + Problems (本阶段问题挂这里).
	Theme                 string // Theme 实际使用的主题名.
}

// Stage 是渲染阶段: 实现 pipeline.Stage, 供编排层串联; 零值可用 (默认选项).
type Stage struct{}

// Run 执行一次带上下文的渲染 (默认选项).
func (Stage) Run(ctx context.Context, in *derive.Result) (*Result, error) {
	return Render(ctx, in)
}

// Render 使用默认选项执行渲染.
func Render(ctx context.Context, derived *derive.Result) (*Result, error) {
	return RenderWithOptions(ctx, derived, Options{})
}

// RenderWithOptions 与 Render 相同, 支持自定义渲染选项.
func RenderWithOptions(ctx context.Context, derived *derive.Result, opts Options) (*Result, error) {
	if derived == nil {
		return nil, fmt.Errorf("render: nil input")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sp := derived.Space
	if sp == nil {
		return nil, fmt.Errorf("render: nil space")
	}

	theme := opts.Theme
	if theme == "" {
		theme = sp.Config.ThemeName
	}
	if theme == "" {
		theme = "default"
	}

	fsys := opts.FS
	if fsys == nil {
		fsys = os.DirFS(sp.Root)
	}
	themesDir := sp.Layout.Themes
	if themesDir == "" {
		themesDir = "themes"
	}
	themeDir := themesDir + "/" + theme

	var problems []pipeline.Problem
	if _, err := fs.Stat(fsys, themeDir); err != nil {
		problems = append(problems, pipeline.Problemf(pipeline.SeverityWarning, "render", themeDir,
			"主题 %q 不存在或不可读, 使用内置默认布局", theme))
	}

	out := &Result{
		Result: pipeline.Result[Page]{Space: sp},
		Theme:  theme,
	}
	byURL := make(map[string]derive.Page, derived.DocCount())
	for _, p := range derived.Docs {
		byURL[p.URL] = p
	}

	// 布局缓存: kind → 模板文本.
	layouts := make(map[string]string)
	for _, p := range derived.Docs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		kind := pageKind(p)
		layout, ok := layouts[kind]
		if !ok {
			var err error
			layout, _, err = themeLayout(fsys, themeDir, kind)
			if err != nil {
				problems = append(problems, pipeline.Problemf(pipeline.SeverityError, "render", p.RelPath,
					"加载布局 %q 失败: %v", kind, err))
			}
			if layout == "" {
				layout = defaultLayout // 未命中布局或加载失败 → 内置兜底.
			}
			layouts[kind] = layout
		}

		vm := ViewModel{
			SiteTitle:  sp.Config.SiteTitle,
			Title:      derive.PageTitle(p),
			URL:        p.URL,
			Content:    template.HTML(markdown.RenderHTML(p.AST, p.Body)),
			Breadcrumb: crumbs(p.Nav),
			Prev:       linkOf(p.Prev, byURL),
			Next:       linkOf(p.Next, byURL),
		}
		tmpl, err := template.New("page").Parse(layout)
		if err != nil {
			problems = append(problems, pipeline.Problemf(pipeline.SeverityError, "render", p.RelPath,
				"解析布局模板失败: %v", err))
			continue // 该页无法渲染, 跳过并继续.
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, vm); err != nil {
			problems = append(problems, pipeline.Problemf(pipeline.SeverityError, "render", p.RelPath,
				"执行布局模板失败: %v", err))
			continue
		}
		out.Docs = append(out.Docs, Page{
			Page:    p,
			HTML:    buf.Bytes(),
			OutPath: outPathFor(p.URL),
		})
	}

	out.Problems = problems
	return out, nil
}

// ==============================
// 布局解析.
// ==============================

// pageKind 派生页面种类: 根入口 / 段入口 / 普通页, 对应布局候选名.
func pageKind(p derive.Page) string {
	if p.URL == "/" {
		return "index"
	}
	if p.IsIndex {
		return "section"
	}
	return "page"
}

// themeLayout 读取主题布局: 按 <kind>.html → _default.html 的顺序查找.
//
// 返回 (模板文本, 是否命中, 错误): 未命中时调用方使用内置默认布局;
// 真实 IO 错误 (非 fs.ErrNotExist) 以 err 返回.
func themeLayout(fsys fs.FS, themeDir, kind string) (string, bool, error) {
	for _, name := range []string{kind + ".html", "_default.html"} {
		b, err := fs.ReadFile(fsys, themeDir+"/layouts/"+name)
		if err == nil {
			return string(b), true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", true, err
		}
	}
	return "", false, nil
}

// outPathFor 派生输出相对路径: clean URL → <url>/index.html, 根 → index.html.
func outPathFor(url string) string {
	u := strings.Trim(url, "/")
	if u == "" {
		return "index.html"
	}
	return u + "/index.html"
}

// ==============================
// 模板数据模型.
// ==============================

// ViewModel 是模板可用的视图模型 (避免把内部结构直接暴露给模板).
type ViewModel struct {
	SiteTitle  string        // SiteTitle 站点标题 (Config.SiteTitle).
	Title      string        // Title 页面标题.
	URL        string        // URL 页面输出路径.
	Content    template.HTML // Content 渲染后的正文 (Markdown → HTML, 已信任).
	Breadcrumb []Crumb       // Breadcrumb 面包屑.
	Prev       Link          // Prev 上篇链接.
	Next       Link          // Next 下篇链接.
}

// Crumb 是面包屑的一项.
type Crumb struct {
	Title string // Title 展示名.
	URL   string // URL 输出路径.
}

// Link 是上/下篇链接.
type Link struct {
	Title string // Title 展示名; 未知页面为空.
	URL   string // URL 输出路径; 空表示不存在.
}

// crumbs 把派生面包屑转为模板模型.
func crumbs(nav []derive.NavItem) []Crumb {
	if len(nav) == 0 {
		return nil
	}
	out := make([]Crumb, 0, len(nav))
	for _, n := range nav {
		out = append(out, Crumb{Title: n.Title, URL: n.URL})
	}
	return out
}

// linkOf 把 URL 转成带标题的链接 (标题取自页面表; 未知 URL 只有地址).
func linkOf(url string, byURL map[string]derive.Page) Link {
	if url == "" {
		return Link{}
	}
	title := ""
	if p, ok := byURL[url]; ok {
		title = derive.PageTitle(p)
	}
	return Link{Title: title, URL: url}
}

// ==============================
// 内置默认布局 (兜底).
// ==============================

// defaultLayout 是主题/布局缺失时的内置兜底布局, 保证 Space 无需主题也能出站.
const defaultLayout = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8">
<title>{{.SiteTitle}} - {{.Title}}</title>
</head>
<body>
<header>
<h1>{{.Title}}</h1>
{{if .Breadcrumb}}<nav>{{range .Breadcrumb}}<a href="{{.URL}}">{{.Title}}</a>{{end}}</nav>{{end}}
</header>
<main>
{{.Content}}
</main>
{{if or .Prev.URL .Next.URL}}<footer><nav>
{{if .Prev.URL}}<a href="{{.Prev.URL}}">&larr; {{.Prev.Title}}</a>{{end}}
{{if .Next.URL}}<a href="{{.Next.URL}}">{{.Next.Title}} &rarr;</a>{{end}}
</nav></footer>{{end}}
</body>
</html>
`
