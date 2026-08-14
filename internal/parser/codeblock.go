// codeblock.go 自定义代码块渲染: 构建期语法高亮 (chroma) + 语言标识.
// 输出结构:
//   <pre class="chroma code-block" data-language="go"><code>...</code></pre>
// 高亮配色由主题 assets 中的 chroma CSS (github 主题) 提供.
// 未知语言 / 无语言代码块降级为转义纯文本, 仍保留 code-block 样式.
package parser

import (
	"bytes"
	"html"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// highlightTheme 是代码高亮配色 (chroma 内置主题名).
const highlightTheme = "github"

// codeBlockRenderer 覆盖 goldmark 的代码块渲染.
type codeBlockRenderer struct{}

// RegisterFuncs 注册代码块渲染函数.
func (r *codeBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFenced)
	reg.Register(ast.KindCodeBlock, r.renderIndented)
}

// renderFenced 渲染围栏代码块 (info 标注语言).
func (r *codeBlockRenderer) renderFenced(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.FencedCodeBlock)
	writeCodeBlock(w, fencedLang(n, source), codeLines(n, source))
	return ast.WalkContinue, nil
}

// renderIndented 渲染缩进代码块 (无语言标识).
func (r *codeBlockRenderer) renderIndented(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.CodeBlock)
	writeCodeBlock(w, "", codeLines(n, source))
	return ast.WalkContinue, nil
}

// fencedLang 提取围栏代码块的语言: "go {#id .cls}" -> "go".
func fencedLang(n *ast.FencedCodeBlock, source []byte) string {
	if n.Info == nil {
		return ""
	}
	info := strings.TrimSpace(string(n.Info.Segment.Value(source)))
	if i := strings.IndexAny(info, " 	{`"); i >= 0 {
		info = info[:i]
	}
	return info
}

// lineHolder 是包含代码行的节点 (FencedCodeBlock / CodeBlock).
type lineHolder interface {
	Lines() *text.Segments
}

// codeLines 拼接代码块的全部代码行.
func codeLines(n ast.Node, source []byte) string {
	holder, ok := n.(lineHolder)
	if !ok {
		return ""
	}
	var b strings.Builder
	lines := holder.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(source))
	}
	return b.String()
}

// writeCodeBlock 输出代码块 HTML.
// 高亮成功: chroma 输出 <pre class="chroma">, 注入 code-block 类与 data-language;
// 失败 (未知语言 / 渲染错误): 转义纯文本降级, 保留 code-block 样式.
func writeCodeBlock(w util.BufWriter, lang, code string) {
	// chroma 类保留: 高亮 CSS 选择器 (.chroma .kn 等) 依赖它.
	attr := ` class="chroma code-block"`
	if lang != "" {
		attr += ` data-language="` + html.EscapeString(lang) + `"`
	}
	if highlighted, ok := highlightCode(code, lang); ok {
		out := strings.Replace(highlighted, `<pre class="chroma">`, "<pre"+attr+">", 1)
		// 保留 goldmark 惯例的 language-xxx class, 兼容基于它的 CSS 与工具.
		if lang != "" {
			out = strings.Replace(out, `<code>`, `<code class="language-`+html.EscapeString(lang)+`">`, 1)
		}
		w.WriteString(out)
		return
	}
	w.WriteString("<pre" + attr + "><code>")
	w.WriteString(html.EscapeString(code))
	w.WriteString("</code></pre>")
}

// highlightCode 用 chroma 渲染高亮 HTML.
// 返回 chroma 输出的完整 HTML 与是否成功; 语言未知或失败返回 false.
func highlightCode(code, lang string) (string, bool) {
	lexer := lexers.Get(lang)
	if lexer == nil {
		return "", false
	}
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", false
	}
	formatter := chromahtml.New(chromahtml.WithClasses(true))
	style := styles.Get(highlightTheme)
	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return "", false
	}
	return buf.String(), true
}
