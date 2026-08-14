package parser

import (
	"strings"
	"testing"
)

func TestSplitFrontMatter(t *testing.T) {
	content := "---\ntitle: 测试文档\nauthor: Eman\ndate: 2026-08-13\ntag: Plainship\n---\n\n# Hello\n"
	meta, body, err := SplitFrontMatter([]byte(content))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if meta.GetString("title") != "测试文档" {
		t.Errorf("title = %q, 期望 测试文档", meta.GetString("title"))
	}
	if meta.GetString("author") != "Eman" {
		t.Errorf("author = %q", meta.GetString("author"))
	}
	if !strings.Contains(string(body), "# Hello") {
		t.Errorf("正文缺少内容: %q", body)
	}
}

func TestSplitFrontMatter_NoFrontMatter(t *testing.T) {
	content := "# 没有 Front Matter\n正文内容"
	meta, body, err := SplitFrontMatter([]byte(content))
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if len(meta) != 0 {
		t.Errorf("meta 应为空: %v", meta)
	}
	if string(body) != content {
		t.Errorf("body 应与原文一致")
	}
}

func TestSplitFrontMatter_MissingCloseMarker(t *testing.T) {
	content := "---\ntitle: 未闭合\n"
	_, _, err := SplitFrontMatter([]byte(content))
	if err == nil {
		t.Fatal("缺少结束标记应报错")
	}
}

func TestSplitFrontMatter_InvalidYAML(t *testing.T) {
	content := "---\ntitle: [未闭合\n---\n正文"
	_, _, err := SplitFrontMatter([]byte(content))
	if err == nil {
		t.Fatal("无效 YAML 应报错")
	}
}

func TestParse_ChineseFileName(t *testing.T) {
	content := "---\ntitle: 中文标题\n---\n\n正文内容\n"
	doc, err := Parse([]byte(content), "docs/测试文档.md")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if doc.Title != "中文标题" {
		t.Errorf("Title = %q", doc.Title)
	}
	if doc.FileName != "测试文档.md" {
		t.Errorf("FileName = %q", doc.FileName)
	}
	if doc.Stem != "测试文档" {
		t.Errorf("Stem = %q", doc.Stem)
	}
	if !strings.Contains(string(doc.ContentHTML), "正文内容") {
		t.Errorf("ContentHTML 缺少正文")
	}
}

func TestParse_InvalidDate(t *testing.T) {
	content := "---\ntitle: 测试\ndate: 不是日期\n---\n正文"
	_, err := Parse([]byte(content), "docs/bad.md")
	if err == nil {
		t.Fatal("无效日期应报错")
	}
	if !strings.Contains(err.Error(), "date") {
		t.Errorf("错误应包含 date 提示: %v", err)
	}
	if !strings.Contains(err.Error(), "docs/bad.md") {
		t.Errorf("错误应包含文件路径: %v", err)
	}
}

func TestParse_MarkdownFeatures(t *testing.T) {
	content := `---
title: 特性测试
---

# 标题

**粗体** 与 *斜体*

- 列表 A
- 列表 B

1. 有序一
2. 有序二

> 引用

` + "```go" + `
fmt.Println("hello")
` + "```" + `

| 列1 | 列2 |
| --- | --- |
| a | b |

- [ ] 未完成
- [x] 已完成
`
	doc, err := Parse([]byte(content), "docs/features.md")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	html := string(doc.ContentHTML)
	for _, want := range []string{"<strong>", "<em>", "<ul>", "<ol>", "<blockquote>", "language-go", "<table>", "<input"} {
		if !strings.Contains(html, want) {
			t.Errorf("缺少 %s, 实际 HTML: %s", want, html[:min(len(html), 200)])
		}
	}
}

func TestParse_DraftAndLayout(t *testing.T) {
	content := "---\ntitle: 草稿\nlayout: page\ndraft: true\n---\n正文"
	doc, err := Parse([]byte(content), "docs/draft.md")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !doc.Draft {
		t.Error("draft 应为 true")
	}
	if doc.Layout != "page" {
		t.Errorf("layout = %q", doc.Layout)
	}
}

// TestParse_RawHTML 验证默认 (unsafe=false) 时原始 HTML 被转义,
// 开启 unsafe 后原样输出. 防止发布站点 XSS.
func TestParse_RawHTML(t *testing.T) {
	content := "---\ntitle: X\n---\n正文 <script>alert(1)</script> <img src=x onerror=alert(1)>"
	doc, err := Parse([]byte(content), "docs/x.md")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	html := string(doc.ContentHTML)
	if strings.Contains(html, "<script>") {
		t.Errorf("默认模式不应直通 <script>: %s", html)
	}
	// goldmark 非 unsafe 模式把 raw HTML 替换为占位注释.
	if !strings.Contains(html, "raw HTML omitted") {
		t.Errorf("默认模式应剔除 raw HTML: %s", html)
	}

	// 开启 unsafe: 原始 HTML 直通.
	doc2, err := Parse([]byte(content), "docs/x.md", WithUnsafe(true))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	html2 := string(doc2.ContentHTML)
	if !strings.Contains(html2, "<script>alert(1)</script>") {
		t.Errorf("unsafe 模式应直通原始 HTML: %s", html2)
	}
}

func TestParse_UnicodeContent(t *testing.T) {
	content := "---\ntitle: 多语言\n---\n\n中文文本 日本語 English 🎉 #$%&"
	doc, err := Parse([]byte(content), "docs/unicode.md")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if !strings.Contains(string(doc.ContentHTML), "日本語") {
		t.Error("缺少日文内容")
	}
	if !strings.Contains(string(doc.ContentHTML), "🎉") {
		t.Error("缺少 emoji 内容")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
