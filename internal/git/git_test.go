package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupRepo 在临时目录初始化 Git 仓库并配置身份.
func setupRepo(t *testing.T) string {
	t.Helper()
	if !Available() {
		t.Skip("系统未安装 Git, 跳过测试")
	}
	dir := t.TempDir()
	mustRun(t, dir, "init", "--quiet")
	mustRun(t, dir, "config", "user.email", "test@plainship.dev")
	mustRun(t, dir, "config", "user.name", "Plainship Test")
	return dir
}

func mustRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v 失败: %v: %s", args, err, out)
	}
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsRepo(t *testing.T) {
	dir := setupRepo(t)
	if !IsRepo(dir) {
		t.Error("应为 Git 仓库")
	}
	plain := t.TempDir()
	if IsRepo(plain) {
		t.Error("普通目录不应是 Git 仓库")
	}
}

func TestRoot_Found(t *testing.T) {
	dir := setupRepo(t)
	sub := filepath.Join(dir, "docs")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	root, ok := Root(sub)
	if !ok {
		t.Fatal("应从子目录找到仓库根")
	}
	// 以 git 自身从同一子目录解析出的仓库根为基准:
	// 避免 Windows 8.3 短路径 (如 EMANYZ~1) 与长路径的显示差异.
	expected, _, err := run(context.Background(), sub, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	expected = strings.TrimSpace(expected)
	if expected == "" {
		t.Fatal("git rev-parse --show-toplevel 返回空")
	}
	expectedAbs, err := filepath.Abs(expected)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(root), filepath.Clean(expectedAbs)) {
		t.Errorf("root = %s, 期望 %s", root, expectedAbs)
	}
}

func TestBranch(t *testing.T) {
	dir := setupRepo(t)
	// 创建首个提交, 确保分支已建立.
	write(t, dir, "README.md", "内容")
	mustRun(t, dir, "add", "-A")
	mustRun(t, dir, "commit", "-m", "init", "--quiet")
	branch := Branch(dir)
	if branch == "" {
		t.Error("应能读取分支名")
	}
}

func TestBranch_Unborn(t *testing.T) {
	dir := setupRepo(t)
	// 无提交 (unborn HEAD) 时 rev-parse 失败, 应回退 symbolic-ref 返回分支名.
	branch := Branch(dir)
	if branch == "" {
		t.Fatal("无提交仓库也应返回分支名 (unborn HEAD)")
	}
	if strings.HasPrefix(branch, "refs/") {
		t.Errorf("应返回纯分支名, 实际 %q", branch)
	}
	sym, _, err := run(context.Background(), dir, "symbolic-ref", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimPrefix(strings.TrimSpace(sym), "refs/heads/")
	if branch != want {
		t.Errorf("Branch() = %q, 期望 %q", branch, want)
	}
}

func TestPorcelain_ChineseFileNames(t *testing.T) {
	dir := setupRepo(t)
	write(t, dir, "docs/测试文档.md", "内容")
	mustRun(t, dir, "add", "-A")
	mustRun(t, dir, "commit", "-m", "init", "--quiet")
	// 修改文件, 使 porcelain 输出包含具体路径.
	write(t, dir, "docs/测试文档.md", "修改后")
	entries, err := Porcelain(dir)
	if err != nil {
		t.Fatalf("Porcelain 失败: %v", err)
	}
	found := false
	for _, e := range entries {
		if strings.Contains(e.Path, "测试文档.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("未检测到中文文件名: %+v", entries)
	}
}

func TestFileChanges_AddedModifiedDeleted(t *testing.T) {
	dir := setupRepo(t)
	write(t, dir, "a.md", "内容 A")
	write(t, dir, "b.md", "内容 B")
	mustRun(t, dir, "add", "-A")
	mustRun(t, dir, "commit", "-m", "init", "--quiet")

	// 新增.
	write(t, dir, "c.md", "内容 C")
	// 修改.
	write(t, dir, "a.md", "修改后的 A")
	// 删除.
	mustRun(t, dir, "rm", "b.md")

	added, modified, deleted, err := FileChanges(dir)
	if err != nil {
		t.Fatalf("FileChanges 失败: %v", err)
	}
	if len(added) != 1 || len(modified) != 1 || len(deleted) != 1 {
		t.Errorf("added=%v modified=%v deleted=%v", added, modified, deleted)
	}
}

func TestClean(t *testing.T) {
	dir := setupRepo(t)
	write(t, dir, "a.md", "内容")
	mustRun(t, dir, "add", "-A")
	mustRun(t, dir, "commit", "-m", "init", "--quiet")
	if !Clean(dir) {
		t.Error("提交后应 clean")
	}
	write(t, dir, "a.md", "改动")
	if Clean(dir) {
		t.Error("修改后不应 clean")
	}
}

func TestInit_Idempotent(t *testing.T) {
	dir := setupRepo(t)
	// 已存在仓库时不应报错.
	if err := Init(dir); err != nil {
		t.Errorf("对已有仓库 Init 不应报错: %v", err)
	}
}
