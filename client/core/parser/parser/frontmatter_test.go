package parser

import (
	"bytes"
	"strings"
	"testing"
)

// TestSplitFrontMatterNone 锁定无 Front Matter 的形态: 整篇即正文.
func TestSplitFrontMatterNone(t *testing.T) {
	content := []byte("# Hi\n\nSome text\n")
	meta, body, has, closed := splitFrontMatter(content)
	if has || closed {
		t.Fatalf("has=%v closed=%v, want false/false", has, closed)
	}
	if meta != nil {
		t.Errorf("meta = %q, want nil", meta)
	}
	if !bytes.Equal(body, content) {
		t.Errorf("body = %q, want content %q (无 FM 时正文保持原样)", body, content)
	}
}

// TestSplitFrontMatterSimple 锁定标准切分: 闭合行与正文的位置关系.
func TestSplitFrontMatterSimple(t *testing.T) {
	content := []byte("---\ntitle: demo\n---\n# Hi\n")
	meta, body, has, closed := splitFrontMatter(content)
	if !has || !closed {
		t.Fatalf("has=%v closed=%v, want true/true", has, closed)
	}
	if string(meta) != "title: demo\n" {
		t.Errorf("meta = %q, want %q (块内容含行尾换行)", meta, "title: demo\n")
	}
	if string(body) != "# Hi\n" {
		t.Errorf("body = %q, want %q", body, "# Hi\n")
	}
}

// TestSplitFrontMatterEmpty 锁定空 Front Matter: 立即闭合.
func TestSplitFrontMatterEmpty(t *testing.T) {
	content := []byte("---\n---\nbody\n")
	meta, body, has, closed := splitFrontMatter(content)
	if !has || !closed {
		t.Fatalf("has=%v closed=%v, want true/true", has, closed)
	}
	if len(meta) != 0 {
		t.Errorf("meta = %q, want empty", meta)
	}
	if string(body) != "body\n" {
		t.Errorf("body = %q, want %q", body, "body\n")
	}
}

// TestSplitFrontMatterUnclosed 锁定未闭合: has=true closed=false, 正文=整篇原文.
func TestSplitFrontMatterUnclosed(t *testing.T) {
	content := []byte("---\ntitle: demo\n")
	meta, body, has, closed := splitFrontMatter(content)
	if !has || closed {
		t.Fatalf("has=%v closed=%v, want true/false", has, closed)
	}
	if meta != nil {
		t.Errorf("meta = %q, want nil", meta)
	}
	if !bytes.Equal(body, content) {
		t.Errorf("body = %q, want original content (未闭合时正文回退整篇)", body)
	}
}

// TestSplitFrontMatterBodyContainsDelimiter 锁定: 正文中的 --- 不影响切分.
func TestSplitFrontMatterBodyContainsDelimiter(t *testing.T) {
	content := []byte("---\ntitle: demo\n---\n# A\n---\n# B\n")
	meta, body, _, closed := splitFrontMatter(content)
	if !closed {
		t.Fatal("closed = false, want true")
	}
	if string(meta) != "title: demo\n" {
		t.Errorf("meta = %q, want %q (块内容含行尾换行)", meta, "title: demo\n")
	}
	if string(body) != "# A\n---\n# B\n" {
		t.Errorf("body = %q, want %q (正文完整保留)", body, "# A\n---\n# B\n")
	}
}

// TestSplitFrontMatterBlankFirstLine 锁定: 首行有前导空白/结尾空格仍识别为分隔行.
func TestSplitFrontMatterBlankFirstLine(t *testing.T) {
	content := []byte("  ---  \ntitle: demo\n---\n# Hi\n")
	meta, _, has, closed := splitFrontMatter(content)
	if !has || !closed {
		t.Fatalf("has=%v closed=%v, want true/true", has, closed)
	}
	if string(meta) != "title: demo\n" {
		t.Errorf("meta = %q, want %q (块内容含行尾换行)", meta, "title: demo\n")
	}
}

// TestSplitFrontMatterCRLF 锁定 CRLF 行尾 (Windows): 切分与解码均正常.
func TestSplitFrontMatterCRLF(t *testing.T) {
	content := []byte("---\r\ntitle: demo\r\ndraft: true\r\n---\r\n# Hi\r\n")
	meta, body, has, closed := splitFrontMatter(content)
	if !has || !closed {
		t.Fatalf("has=%v closed=%v, want true/true", has, closed)
	}
	if !strings.Contains(string(meta), "draft: true") {
		t.Errorf("meta = %q, want to contain draft: true", meta)
	}
	if !strings.HasPrefix(string(body), "# Hi") {
		t.Errorf("body = %q, want to start with # Hi", body)
	}
}

// TestDecodeMeta 锁定 YAML 解码边界.
func TestDecodeMeta(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantVal any
		wantErr bool
	}{
		{"空白块", "\n  \n", "", false},
		{"简单键值", "title: Hi\n", "Hi", false},
		{"列表值", "tags:\n - a\n - b\n", nil, false},
		{"嵌套映射", "author:\n  name: Bob\n", nil, false},
		{"标量不是映射", "just a string\n", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := decodeMeta([]byte(c.raw))
			if c.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeMeta: %v", err)
			}
			if c.wantVal == "" && c.wantVal != nil {
				// 空白块: 期待空 map.
				if len(m) != 0 {
					t.Errorf("empty block meta = %v, want empty map", m)
				}
				return
			}
			if v, ok := m["title"]; ok && v != c.wantVal {
				t.Errorf("title = %v, want %v", v, c.wantVal)
			}
		})
	}
}
