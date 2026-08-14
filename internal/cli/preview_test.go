package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/plainship/internal/clifx"
	"github.com/emanyzwww/plainship/internal/i18n"
)

// TestPreviewHandler 服务 build 目录内容 (index.html 可访问).
func TestPreviewHandler(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>hi</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(previewHandler(dir))
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

// TestPreviewPlan_NotBuilt build 目录缺失时报错.
func TestPreviewPlan_NotBuilt(t *testing.T) {
	dir := t.TempDir()
	_, err := previewPlan(dir)
	if err == nil {
		t.Fatal("build/ 不存在时应报错")
	}
	if !strings.Contains(err.Error(), "build/ does not exist") {
		t.Errorf("错误信息 = %v", err)
	}
}

// TestPreviewPlan_Suggest 未构建时建议先 build.
func TestPreviewPlan_Suggest(t *testing.T) {
	dir := t.TempDir()
	err := previewPlanError(t, dir)
	if got := clifx.SuggestFor(err); got != i18n.SuggestBuildFirst {
		t.Errorf("建议 = %q, want %q", got, i18n.SuggestBuildFirst)
	}
}

// previewPlanError 辅助: 返回 previewPlan 的错误.
func previewPlanError(t *testing.T, dir string) error {
	t.Helper()
	_, err := previewPlan(dir)
	if err == nil {
		t.Fatal("previewPlan 应报错")
	}
	return err
}

// TestCLI_Preview_NotBuilt 未构建时 preview 命令报错.
func TestCLI_Preview_NotBuilt(t *testing.T) {
	dir := t.TempDir()
	if _, err := runCLI(t, dir, "new", dir); err != nil {
		t.Fatalf("new 失败: %v", err)
	}
	if _, err := runCLI(t, dir, "preview"); err == nil {
		t.Error("未构建时 preview 应报错")
	}
}

// TestCLI_Preview_Help preview 出现在帮助中.
func TestCLI_Preview_Help(t *testing.T) {
	out, err := runCLI(t, t.TempDir(), "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "preview") {
		t.Errorf("帮助缺少 preview 命令: %s", out)
	}
}
