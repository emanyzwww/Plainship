package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestAskConfirm 确认输入解析: y/yes 通过, 其余拒绝, EOF 拒绝.
func TestAskConfirm(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"Y\n", true},
		{"YES\n", true},
		{"\n", false},
		{"n\n", false},
		{"N\n", false},
		{"no\n", false},
		{"maybe\n", false},
	}
	for _, c := range cases {
		var out bytes.Buffer
		if got := askConfirm(strings.NewReader(c.in), "> ", &out); got != c.want {
			t.Errorf("askConfirm(%q) = %v, want %v", c.in, got, c.want)
		}
		if !strings.Contains(out.String(), "> ") {
			t.Errorf("askConfirm 应输出提示, out = %q", out.String())
		}
	}
	// EOF (无输入) → false.
	var out bytes.Buffer
	if got := askConfirm(strings.NewReader(""), "> ", &out); got {
		t.Error("EOF 应返回 false")
	}
}

// TestCLI_Publish_YesFlag --yes flag 被接受且不改变原有守卫行为.
func TestCLI_Publish_YesFlag(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	if _, err := runCLI(t, dir, "publish", "--yes"); err == nil {
		t.Error("未配置服务器时 publish --yes 也应报错")
	}
}
