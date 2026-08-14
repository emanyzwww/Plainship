package format

import (
	"strings"
	"testing"
)

// TestChain 验证纯链式调用: 从 NewLine() 开始一气呵成, 不经过中间变量.
func TestChain(t *testing.T) {
	s := NewLine().
		Text("title").Br().Br().
		Text("Usage:").Br().
		Indent(2).Text("plainship publish").Tab().Text("Publish to the server").Br().
		String()
	want := "title\n\nUsage:\n  plainship publish  Publish to the server\n"
	if s != want {
		t.Errorf("chain = %q, want %q", s, want)
	}
}

// TestAlignColumns 验证 tabwriter 自动对齐 (含 CJK 显示宽度).
func TestAlignColumns(t *testing.T) {
	s := NewLine().
		Text("  plainship new <路径>").Tab().Text("Create a new Space").Br().
		Text("  plainship build [-m 消息]").Tab().Text("Build and commit").Br().
		Text("  plainship publish").Tab().Text("Publish").Br().
		String()
	lines := strings.Split(s, "\n")
	if len(lines) != 4 {
		t.Fatalf("行数 = %d, want 4", len(lines))
	}
	// 用显示宽度列 (视觉列) 比较: CJK 字符占 2 列.
	col := func(line, sub string) int {
		idx := strings.Index(line, sub)
		if idx < 0 {
			return -1
		}
		return displayWidth(line[:idx])
	}
	c1, c2, c3 := col(lines[0], "Create"), col(lines[1], "Build"), col(lines[2], "Publish")
	if c1 != c2 || c2 != c3 {
		t.Errorf("列未对齐: %d, %d, %d\n%s", c1, c2, c3, s)
	}
	if c1 < 20 {
		t.Errorf("对齐列过早: %d", c1)
	}
}

// TestStringIdempotent 验证 String 可重复调用且结果一致.
func TestStringIdempotent(t *testing.T) {
	l := NewLine().Text("a").Br().Text("b").Br()
	first := l.String()
	second := l.String()
	if first != second {
		t.Errorf("String 非幂等: %q vs %q", first, second)
	}
}

// TestEmpty 验证空行与 Textf 与 Indent.
func TestEmptyAndTextf(t *testing.T) {
	s := NewLine().
		Textf("v%s", "0.1.2").Br().
		Empty().
		Indent(4).Text("x").Br().
		String()
	want := "v0.1.2\n\n    x\n"
	if s != want {
		t.Errorf("got %q, want %q", s, want)
	}
}

// TestEmptyBuilder 验证空构建器输出为空字符串.
func TestEmptyBuilder(t *testing.T) {
	if s := NewLine().String(); s != "" {
		t.Errorf("空构建器 = %q", s)
	}
}
