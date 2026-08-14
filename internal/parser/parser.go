// Package parser 负责解析 Markdown 文档与 YAML Front Matter.
// Markdown 解析使用成熟的 goldmark 库, 不自行实现.
package parser

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/emanyzwww/Plainship/internal/i18n"
	"github.com/emanyzwww/Plainship/internal/layout"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"gopkg.in/yaml.v3"

	"github.com/emanyzwww/Plainship/internal/model"
)

// ErrNoCloseMarker 表示 Front Matter 缺少结束标记.
var ErrNoCloseMarker = i18n.Errorf(i18n.ParserErrNoClose)

// SplitFrontMatter 将原始文件内容拆分为 metadata 与正文.
// 文件必须以 --- 开头才视为包含 Front Matter.
// 返回的 body 不包含 Front Matter 部分.
func SplitFrontMatter(content []byte) (model.Metadata, []byte, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	lines := bytes.Split(normalized, []byte("\n"))
	if len(lines) == 0 || strings.TrimSpace(string(lines[0])) != "---" {
		return model.Metadata{}, normalized, nil
	}
	// 查找结束标记 ---.
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(string(lines[i])) == "---" {
			endIdx = i
			break
		}
	}
	if endIdx == -1 {
		return nil, nil, ErrNoCloseMarker
	}
	fmLines := lines[1:endIdx]
	body := bytes.Join(lines[endIdx+1:], []byte("\n"))
	meta := model.Metadata{}
	if len(fmLines) > 0 {
		if err := yaml.Unmarshal(bytes.Join(fmLines, []byte("\n")), &meta); err != nil {
			return nil, nil, i18n.Errorf(i18n.ParserErrYAML, err)
		}
	}
	return meta, body, nil
}

// SplitFrontMatterFile 读取文件并只解析 Front Matter.
// 用于路由预解析, 避免完整解析 Markdown.
func SplitFrontMatterFile(path string) (model.Metadata, []byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return SplitFrontMatter(content)
}

// Option 是解析选项.
type Option func(*parseOptions)

type parseOptions struct {
	// resolveLink 用于将 Markdown 链接解析为最终 URL.
	// 参数为源文件相对路径与链接目标, 返回解析后的 URL.
	resolveLink func(srcRel, dest string) string
	// unsafe 为 true 时允许正文中的原始 HTML 直接输出.
	// 默认 false: raw HTML 会被转义, 防止发布站点 XSS.
	unsafe bool
}

// WithLinkResolver 注入链接解析函数.
func WithLinkResolver(fn func(srcRel, dest string) string) Option {
	return func(o *parseOptions) {
		o.resolveLink = fn
	}
}

// WithUnsafe 控制是否允许原始 HTML 直通输出 (对应 goldmark html.WithUnsafe).
func WithUnsafe(v bool) Option {
	return func(o *parseOptions) {
		o.unsafe = v
	}
}

// linkTransformer 在 AST 阶段重写 Markdown 链接.
type linkTransformer struct {
	srcRel  string
	resolve func(srcRel, dest string) string
}

// Transform 实现 parser.ASTTransformer 接口.
func (t *linkTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	if t.resolve == nil {
		return
	}
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch l := n.(type) {
		case *ast.Link:
			l.Destination = []byte(t.resolve(t.srcRel, string(l.Destination)))
		case *ast.Image:
			l.Destination = []byte(t.resolve(t.srcRel, string(l.Destination)))
		}
		return ast.WalkContinue, nil
	})
}

// extractSummary 从正文文本中提取第一个段落的纯文本摘要.
// 跳过标题, 代码块与空行. 去除行内 Markdown 标记, 截断到 160 字.
func extractSummary(body []byte) string {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	var para []string
	inFence := false
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(t, "#") {
			if len(para) > 0 {
				break
			}
			continue
		}
		if t == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		para = append(para, t)
	}
	if len(para) == 0 {
		return ""
	}
	summary := stripInline(strings.Join(para, " "))
	runes := []rune(summary)
	if len(runes) > 160 {
		return string(runes[:160]) + "..."
	}
	return summary
}

// stripInline 去除行内 Markdown 标记.
func stripInline(s string) string {
	// 处理 [text](url) 与 ![alt](url) 形式的链接与图片.
	reLink := regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	s = reLink.ReplaceAllString(s, "$1")
	// 去除强调与代码标记.
	for _, marker := range []string{"**", "__", "`", "*", "_"} {
		s = strings.ReplaceAll(s, marker, "")
	}
	return strings.TrimSpace(s)
}

// Parse 解析一篇 Markdown 文档, 返回 Document 模型.
// sourceRel 是相对 Space 根目录的路径, 例如 docs/测试文档.md.
// 该函数不负责路由解析, 只提取 Front Matter 字段.
func Parse(content []byte, sourceRel string, opts ...Option) (*model.Document, error) {
	o := &parseOptions{}
	for _, opt := range opts {
		opt(o)
	}
	meta, body, err := SplitFrontMatter(content)
	if err != nil {
		return nil, i18n.Errorf(i18n.ParserErrFile, sourceRel, err)
	}
	doc := &model.Document{
		SourcePath: sourceRel,
		FileName:   filepath.Base(sourceRel),
		Stem:       strings.TrimSuffix(filepath.Base(sourceRel), filepath.Ext(sourceRel)),
		Meta:       meta,
		RawContent: string(body),
	}
	doc.Dir = dirOfSource(sourceRel)
	doc.Title = meta.GetString("title")
	if doc.Title == "" {
		doc.Title = doc.Stem
	}
	doc.Author = meta.GetString("author")
	doc.Tag = meta.GetString("tag")
	doc.Slug = meta.GetString("slug")
	doc.Layout = meta.GetString("layout")
	doc.Draft = meta.GetBool("draft")
	doc.Date = meta.GetString("date")
	if doc.Date == "" {
		doc.Date = meta.GetString("updated")
	}
	if err := validateDate(doc.Date, sourceRel, content); err != nil {
		return nil, err
	}

	// 构建 goldmark 渲染器.
	md := newMarkdown(sourceRel, o)
	reader := text.NewReader(body)
	root := md.Parser().Parse(reader)
	doc.Summary = extractSummary(body)
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, body, root); err != nil {
		return nil, i18n.Errorf(i18n.ParserErrRender, sourceRel, err)
	}
	doc.ContentHTML = template.HTML(buf.String())
	return doc, nil
}

// RenderMarkdown 将一段 Markdown 正文渲染为 HTML 字符串.
// 供主题或未来扩展直接使用.
func RenderMarkdown(body []byte, srcRel string, resolve func(srcRel, dest string) string, opts ...Option) (string, error) {
	o := &parseOptions{}
	for _, opt := range opts {
		opt(o)
	}
	md := newMarkdown(srcRel, o)
	var buf bytes.Buffer
	if err := md.Convert(body, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// newMarkdown 按选项构建 goldmark 渲染器.
// raw HTML 默认转义, 只有显式开启 unsafe 时才直通.
func newMarkdown(srcRel string, o *parseOptions) goldmark.Markdown {
	rendererOpts := []renderer.Option{}
	if o.unsafe {
		rendererOpts = append(rendererOpts, html.WithUnsafe())
	}
	// 代码块渲染器注册 priority 低于 goldmark 默认 (1000):
	// 注册顺序按 priority 升序, 后注册者覆盖默认渲染器.
	rendererOpts = append(rendererOpts,
		renderer.WithNodeRenderers(util.Prioritized(&codeBlockRenderer{}, 1)))
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(&linkTransformer{srcRel: srcRel, resolve: o.resolveLink}, 100))),
		goldmark.WithRendererOptions(rendererOpts...),
	)
}

// validateDate 校验 date 字段格式.
// 允许空值. 支持 YYYY-MM-DD 与 RFC3339.
func validateDate(date, sourceRel string, fullContent []byte) error {
	if date == "" {
		return nil
	}
	formats := []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05", "2006/01/02"}
	for _, f := range formats {
		if _, err := time.Parse(f, date); err == nil {
			return nil
		}
	}
	// 定位 date 字段所在行, 提供行号提示.
	line := findKeyLine(fullContent, "date")
	pos := ""
	if line > 0 {
		pos = i18n.T(i18n.ParserErrDatePos, line)
	}
	return i18n.Errorf(i18n.ParserErrDate, sourceRel, pos, date)
}

// findKeyLine 在文件内容中查找某个 YAML key 所在的行号(从 1 开始).
func findKeyLine(content []byte, key string) int {
	lines := bytes.Split(content, []byte("\n"))
	for i, l := range lines {
		trimmed := strings.TrimSpace(string(l))
		if strings.HasPrefix(trimmed, key+":") {
			return i + 1
		}
	}
	return 0
}

// dirOfSource 从 sourceRel 中提取目录部分.
// docs/guide/foo.md -> guide, docs/foo.md -> "".
func dirOfSource(sourceRel string) string {
	rel := strings.TrimPrefix(sourceRel, layout.DocsDir+"/")
	dir := filepath.ToSlash(filepath.Dir(rel))
	if dir == "." {
		return ""
	}
	return dir
}
