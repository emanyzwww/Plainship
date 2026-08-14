package parser

import (
	"strings"
	"testing"
)

// TestCodeBlock_Highlighted 围栏代码块输出高亮结构与语言标识.
func TestCodeBlock_Highlighted(t *testing.T) {
	doc := "```go\npackage main\n\nfunc main() {}\n```\n"
	out, err := RenderMarkdown([]byte(doc), "docs/a.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "class=\"chroma code-block\" data-language=\"go\"") {
		t.Errorf("缺少语言标识: %s", out)
	}
	// chroma 高亮: go 关键字带 class.
	if !strings.Contains(out, "<span class=\"kn\">") {
		t.Errorf("缺少高亮 span: %s", out)
	}
	// 原始代码文本保留 (被 span 拆分, 分段断言).
	if !strings.Contains(out, "package") || !strings.Contains(out, "main") {
		t.Errorf("代码内容缺失: %s", out)
	}
}

// TestCodeBlock_NoLang 无语言围栏: 无 data-language, 纯文本降级.
func TestCodeBlock_NoLang(t *testing.T) {
	doc := "```\nplain text\n```\n"
	out, err := RenderMarkdown([]byte(doc), "docs/a.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "class=\"chroma code-block\"") {
		t.Errorf("缺少 code-block 类: %s", out)
	}
	if strings.Contains(out, "data-language") {
		t.Errorf("无语言不应有 data-language: %s", out)
	}
	// 无高亮 span.
	if strings.Contains(out, "<span class=\"kn\">") {
		t.Errorf("无语言不应高亮: %s", out)
	}
}

// TestCodeBlock_UnknownLang 未知语言: 显示语言名但降级纯文本.
func TestCodeBlock_UnknownLang(t *testing.T) {
	doc := "```foobar\nsome code\n```\n"
	out, err := RenderMarkdown([]byte(doc), "docs/a.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "data-language=\"foobar\"") {
		t.Errorf("未知语言应保留语言名: %s", out)
	}
	if strings.Contains(out, "<span class=\"kn\">") {
		t.Errorf("未知语言不应高亮: %s", out)
	}
}

// TestCodeBlock_Escaped 代码内容 HTML 转义.
func TestCodeBlock_Escaped(t *testing.T) {
	doc := "```html\n<div class=\"x\">&</div>\n```\n"
	out, err := RenderMarkdown([]byte(doc), "docs/a.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "<div class=\"x\">") {
		t.Errorf("代码内容未转义: %s", out)
	}
	if !strings.Contains(out, "&lt;") || !strings.Contains(out, "&#34;") {
		t.Errorf("缺少转义输出: %s", out)
	}
}

// TestCodeBlock_Indented 缩进代码块: 无语言, 纯文本降级.
func TestCodeBlock_Indented(t *testing.T) {
	doc := "    indented code\n"
	out, err := RenderMarkdown([]byte(doc), "docs/a.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "class=\"chroma code-block\"") {
		t.Errorf("缩进代码块应有 code-block: %s", out)
	}
	if strings.Contains(out, "data-language") {
		t.Errorf("缩进代码块不应有语言: %s", out)
	}
	if !strings.Contains(out, "indented code") {
		t.Errorf("缩进代码内容缺失: %s", out)
	}
}

// TestCodeBlock_LangWithAttributes info 含属性时只取语言.
func TestCodeBlock_LangWithAttributes(t *testing.T) {
	doc := "```go {#id .cls}\nvar x = 1\n```\n"
	out, err := RenderMarkdown([]byte(doc), "docs/a.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "data-language=\"go\"") {
		t.Errorf("应提取语言 go: %s", out)
	}
}

// TestCodeBlock_LineStructure 保留代码块换行与缩进结构.
func TestCodeBlock_LineStructure(t *testing.T) {
	doc := "```go\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n```\n"
	out, err := RenderMarkdown([]byte(doc), "docs/a.md", nil)
	if err != nil {
		t.Fatal(err)
	}
	// tab 缩进必须保留 (chroma 可能把 tab 单独包进 span).
	if !strings.Contains(out, "\t") {
		t.Errorf("tab 缩进丢失: %s", out)
	}
	// fmt.Println 被 chroma span 拆分, 分段断言.
	if !strings.Contains(out, "fmt") || !strings.Contains(out, "Println") {
		t.Errorf("代码内容缺失: %s", out)
	}
}
