package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/plainship/internal/config"
	"github.com/emanyzwww/plainship/internal/git"
	"github.com/emanyzwww/plainship/internal/revision"
	"github.com/emanyzwww/plainship/internal/space"
	"github.com/emanyzwww/plainship/internal/state"
)

// setupSpace 创建临时 Space 并配置 Git 身份.
func setupSpace(t *testing.T) *space.Space {
	t.Helper()
	root := t.TempDir()
	s, err := space.Create(root, nil)
	if err != nil {
		t.Fatalf("创建 Space 失败: %v", err)
	}
	if _, _, err := git.PassThrough(root, "config", "user.email", "test@plainship.dev"); err != nil {
		t.Fatalf("设置 user.email 失败: %v", err)
	}
	if _, _, err := git.PassThrough(root, "config", "user.name", "Plainship Test"); err != nil {
		t.Fatalf("设置 user.name 失败: %v", err)
	}
	return s
}

// writeDoc 写入 docs 下的文档, 带合法 Front Matter.
func writeDoc(t *testing.T, s *space.Space, rel, title string) {
	t.Helper()
	path := filepath.Join(s.DocsDir(), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ntitle: " + title + "\ndate: 2026-08-13\n---\n\n正文内容.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// logSubjects 返回 git log 的主题列表, 新到旧.
func logSubjects(t *testing.T, dir string) []string {
	t.Helper()
	out, _, err := git.PassThrough(dir, "log", "--format=%s")
	if err != nil {
		t.Fatalf("git log 失败: %v", err)
	}
	var subjects []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			subjects = append(subjects, line)
		}
	}
	return subjects
}

func TestBuild_DocsCommitAndTag(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "测试文档.md", "测试文档")

	res, err := Build(s.Root, BuildOptions{}, nil)
	if err != nil {
		t.Fatalf("Build 失败: %v", err)
	}
	if res.BuildNumber != "ps-0001" {
		t.Errorf("BuildNumber = %s, 期望 ps-0001", res.BuildNumber)
	}
	subjects := logSubjects(t, s.Root)
	// 首次构建: docs/theme/config 三类全部为新增, 分三步提交.
	if len(subjects) != 3 {
		t.Fatalf("首次构建应产生 3 个提交, 实际 %d: %v", len(subjects), subjects)
	}
	wantPrefix := []string{
		"docs build=ps-0001 hash=",
		"theme build=ps-0001 hash=",
		"config build=ps-0001 hash=",
	}
	for i, prefix := range wantPrefix {
		if !strings.HasPrefix(subjects[i], prefix) {
			t.Errorf("第 %d 个提交 = %q, 期望前缀 %q", i, subjects[i], prefix)
		}
	}
	if !strings.HasPrefix(subjects[0], "docs build=ps-0001 hash=") {
		t.Errorf("提交主题不是机器格式: %s", subjects[0])
	}
	tags, _, _ := git.PassThrough(s.Root, "tag", "--list", "ps-*")
	if strings.TrimSpace(tags) != "ps-0001" {
		t.Errorf("tag 列表 = %q", tags)
	}
	if !fileExists(filepath.Join(s.BuildDir(), "index.html")) {
		t.Error("缺少 build/index.html")
	}
	// 状态中应记录编号与类别指纹.
	bs, err := state.LoadState(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	if bs.BuildNumber != "ps-0001" {
		t.Errorf("state.BuildNumber = %s", bs.BuildNumber)
	}
	if len(bs.CategoryHashes) != 3 {
		t.Errorf("state.CategoryHashes = %v", bs.CategoryHashes)
	}
}

func TestBuild_SplitCommitsAllCategories(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "测试文档.md", "测试文档")
	if _, err := Build(s.Root, BuildOptions{}, nil); err != nil {
		t.Fatalf("首次 Build 失败: %v", err)
	}

	// 同时修改 config / theme / docs.
	c, _, _ := config.Load(s.Root, nil)
	c.SetSpaceRoot(s.Root)
	_ = c.SpaceSite.SiteTitle.Set("新标题")
	if _, err := config.Save(c, config.SaveSpace); err != nil {
		t.Fatal(err)
	}
	cssPath := filepath.Join(s.ThemesDir(), "default", "assets", "app.css")
	f, err := os.OpenFile(cssPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n/* 主题变更 */\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	writeDoc(t, s, "新文档.md", "新文档")

	res, err := Build(s.Root, BuildOptions{Message: "发布新文档"}, nil)
	if err != nil {
		t.Fatalf("二次 Build 失败: %v", err)
	}
	if res.BuildNumber != "ps-0002" {
		t.Errorf("BuildNumber = %s, 期望 ps-0002", res.BuildNumber)
	}
	subjects := logSubjects(t, s.Root)
	// 新到旧: docs -> theme -> config, 再往前是首次的 docs.
	if len(subjects) < 4 {
		t.Fatalf("提交数不足: %v", subjects)
	}
	wantPrefix := []string{
		"docs build=ps-0002 hash=",
		"theme build=ps-0002 hash=",
		"config build=ps-0002 hash=",
	}
	for i, prefix := range wantPrefix {
		if !strings.HasPrefix(subjects[i], prefix) {
			t.Errorf("第 %d 个提交 = %q, 期望前缀 %q", i, subjects[i], prefix)
		}
	}
	// -m 消息写入 docs 提交 body.
	bodyOut, _, _ := git.PassThrough(s.Root, "log", "-1", "--format=%B")
	if !strings.Contains(bodyOut, "发布新文档") {
		t.Errorf("-m 消息未写入 docs body: %s", bodyOut)
	}
	// 编号 tag 指向最新提交.
	tags, _, _ := git.PassThrough(s.Root, "tag", "--points-at", "HEAD")
	if !strings.Contains(tags, "ps-0002") {
		t.Errorf("HEAD 缺少 ps-0002 tag: %s", tags)
	}
}

func TestBuild_FailureNoCommit(t *testing.T) {
	s := setupSpace(t)
	path := filepath.Join(s.DocsDir(), "bad.md")
	if err := os.WriteFile(path, []byte("---\ntitle: bad\ndate: 不是日期\n---\n内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(s.Root, BuildOptions{}, nil); err == nil {
		t.Fatal("非法日期应导致 Build 失败")
	}
	subjects := logSubjectsOrEmpty(t, s.Root)
	if len(subjects) != 0 {
		t.Errorf("构建失败后不应有提交: %v", subjects)
	}
	tags, _, _ := git.PassThrough(s.Root, "tag", "--list", "ps-*")
	if strings.TrimSpace(tags) != "" {
		t.Errorf("构建失败后不应有编号: %s", tags)
	}
}

func TestPublish_RefusesUncommitted(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "测试文档.md", "测试文档")
	c, _, _ := config.Load(s.Root, nil)
	c.SetSpaceRoot(s.Root)
	_ = c.SpaceSite.ServerURL.Set("http://127.0.0.1:1")
	if _, err := config.Save(c, config.SaveSpace); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(s.Root, BuildOptions{}, nil); err != nil {
		t.Fatalf("Build 失败: %v", err)
	}
	// 修改文档但不提交.
	writeDoc(t, s, "测试文档.md", "修改后的标题")
	if _, err := Publish(s.Root, nil); err == nil {
		t.Fatal("有未提交变更时 Publish 应拒绝")
	}
}

func TestPublish_RefusesNotBuilt(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "测试文档.md", "测试文档")
	c, _, _ := config.Load(s.Root, nil)
	c.SetSpaceRoot(s.Root)
	_ = c.SpaceSite.ServerURL.Set("http://127.0.0.1:1")
	if _, err := config.Save(c, config.SaveSpace); err != nil {
		t.Fatal(err)
	}
	// 手动提交源码, 从未 build.
	if _, _, err := git.PassThrough(s.Root, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := git.PassThrough(s.Root, "commit", "-m", "docs: manual"); err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(s.Root, nil); err == nil {
		t.Fatal("未构建时 Publish 应拒绝")
	}
}

func TestPublish_RefusesRebuildNeeded(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "测试文档.md", "测试文档")
	c, _, _ := config.Load(s.Root, nil)
	c.SetSpaceRoot(s.Root)
	_ = c.SpaceSite.ServerURL.Set("http://127.0.0.1:1")
	if _, err := config.Save(c, config.SaveSpace); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(s.Root, BuildOptions{}, nil); err != nil {
		t.Fatalf("Build 失败: %v", err)
	}
	// 修改源码并手动提交, 状态 clean, 但未重新 build.
	writeDoc(t, s, "测试文档.md", "修改后的标题")
	if _, _, err := git.PassThrough(s.Root, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := git.PassThrough(s.Root, "commit", "-m", "docs: manual update"); err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(s.Root, nil); err == nil {
		t.Fatal("源码已提交但未重新构建时 Publish 应拒绝")
	}
}

func TestCategoryHash_Stable(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "测试文档.md", "测试文档")
	h1, err := revision.CategoryHash(s, revision.CategoryDocs)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := revision.CategoryHash(s, revision.CategoryDocs)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Error("相同内容的指纹应一致")
	}
	writeDoc(t, s, "测试文档.md", "改后标题")
	h3, _ := revision.CategoryHash(s, revision.CategoryDocs)
	if h1 == h3 {
		t.Error("内容变化后指纹应不同")
	}
}

func logSubjectsOrEmpty(t *testing.T, dir string) []string {
	t.Helper()
	out, _, _ := git.PassThrough(dir, "log", "--format=%s")
	var subjects []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			subjects = append(subjects, line)
		}
	}
	return subjects
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
