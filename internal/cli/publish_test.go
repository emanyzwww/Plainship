package cli

import (
	"testing"
)

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
