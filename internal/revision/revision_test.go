package revision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/plainship/internal/git"
	"github.com/emanyzwww/plainship/internal/space"
)

// setupSpace 创建临时 Space 并配置 Git 身份.
func setupSpace(t *testing.T) *space.Space {
	t.Helper()
	s, err := space.Create(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("创建 Space 失败: %v", err)
	}
	if _, _, err := git.PassThrough(s.Root, "config", "user.email", "test@plainship.dev"); err != nil {
		t.Fatalf("设置 user.email 失败: %v", err)
	}
	if _, _, err := git.PassThrough(s.Root, "config", "user.name", "Plainship Test"); err != nil {
		t.Fatalf("设置 user.name 失败: %v", err)
	}
	return s
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGitStatus_ClassifiesCategories(t *testing.T) {
	s := setupSpace(t)
	// 三类各制造一个变更: config / theme / docs.
	writeFile(t, filepath.Join(s.Root, "plainship.yaml"), "site: {}\n")
	writeFile(t, filepath.Join(s.ThemesDir(), "default", "assets", "app.css"), "body {}\n")
	writeFile(t, filepath.Join(s.DocsDir(), "a.md"), "---\ntitle: A\n---\n内容")

	gs := GitStatus(s)
	if !gs.Available || !gs.IsRepo {
		t.Fatalf("Git 状态异常: %+v", gs)
	}
	for _, cat := range Categories {
		if !gs.Changes[cat].HasChanges() {
			t.Errorf("类别 %s 应检测到变更: %+v", cat, gs.Changes[cat])
		}
	}
	if gs.Clean {
		t.Error("有变更时不应 clean")
	}
}

func TestGitStatus_IgnoresBuildAndState(t *testing.T) {
	s := setupSpace(t)
	writeFile(t, filepath.Join(s.DocsDir(), "a.md"), "---\ntitle: A\n---\n内容")
	// `build/` 与 `.plainship/` 不应计入任何类别.
	writeFile(t, filepath.Join(s.BuildDir(), "index.html"), "<html></html>")
	writeFile(t, filepath.Join(s.Root, ".plainship", "state", "build-state.json"), "{}")

	gs := GitStatus(s)
	if len(gs.Changes[CategoryDocs].Paths) != 1 {
		t.Errorf("docs 变更应只有 1 个: %+v", gs.Changes[CategoryDocs])
	}
}

func TestCategoryHash_StableAndChanged(t *testing.T) {
	s := setupSpace(t)
	writeFile(t, filepath.Join(s.DocsDir(), "a.md"), "---\ntitle: A\n---\n内容")
	h1, err := CategoryHash(s, CategoryDocs)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := CategoryHash(s, CategoryDocs)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Error("相同内容的指纹应一致")
	}
	writeFile(t, filepath.Join(s.DocsDir(), "a.md"), "---\ntitle: A\n---\n修改后")
	h3, _ := CategoryHash(s, CategoryDocs)
	if h1 == h3 {
		t.Error("内容变化后指纹应不同")
	}
}

func TestCommitMessage_RoundTrip(t *testing.T) {
	msg := CommitMessage(CategoryDocs, "ps-0003", strings.Repeat("ab", 32))
	if !strings.HasPrefix(msg, "docs build=ps-0003 hash=") {
		t.Errorf("提交信息格式不正确: %s", msg)
	}
	// 指纹应截短到 16 位.
	if !strings.HasSuffix(msg, "hash="+strings.Repeat("ab", 8)) {
		t.Errorf("指纹未截短: %s", msg)
	}

	s := setupSpace(t)
	writeFile(t, filepath.Join(s.DocsDir(), "a.md"), "---\ntitle: A\n---\n内容")
	if err := CommitPaths(s.Root, msg, "docs"); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	buildNum, catHash, ok := LatestCategoryCommit(s, CategoryDocs)
	if !ok {
		t.Fatal("LatestCategoryCommit 未找到提交")
	}
	if buildNum != "ps-0003" {
		t.Errorf("build 编号 = %s", buildNum)
	}
	if len(catHash) != 16 {
		t.Errorf("解析出的指纹 = %q", catHash)
	}
}

func TestNextBuildNumber(t *testing.T) {
	s := setupSpace(t)
	// 先创建一个提交, 使 HEAD 可被 tag 指向.
	writeFile(t, filepath.Join(s.DocsDir(), "a.md"), "---\ntitle: A\n---\n内容")
	if err := CommitPaths(s.Root, "docs build=ps-0001 hash=abc", "docs"); err != nil {
		t.Fatalf("创建提交失败: %v", err)
	}
	n1, err := NextBuildNumber(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	if n1 != "ps-0001" {
		t.Errorf("首个编号 = %s, 期望 ps-0001", n1)
	}
	if err := git.Tag(s.Root, "ps-0001"); err != nil {
		t.Fatal(err)
	}
	n2, err := NextBuildNumber(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != "ps-0002" {
		t.Errorf("第二个编号 = %s, 期望 ps-0002", n2)
	}
}
