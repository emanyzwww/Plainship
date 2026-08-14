package space

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/plainship/internal/config"
)

func TestCreate_BasicStructure(t *testing.T) {
	root := t.TempDir()
	s, err := Create(root)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	for _, dir := range []string{"docs", "themes"} {
		info, err := os.Stat(filepath.Join(root, dir))
		if err != nil || !info.IsDir() {
			t.Errorf("缺少目录 %s", dir)
		}
	}
	// 不应再生成 config 目录.
	if dirExists(filepath.Join(root, "config")) {
		t.Error("不应存在 config 目录 (配置在根目录)")
	}
	// 主题已生成.
	if !fileExists(filepath.Join(root, "themes", "default", "theme.json")) {
		t.Error("缺少默认主题 theme.json")
	}
	if !fileExists(filepath.Join(root, "themes", "default", "layouts", "article.html")) {
		t.Error("缺少默认主题布局")
	}
	// 配置在根目录生成.
	if !fileExists(filepath.Join(root, config.FileName)) {
		t.Error("缺少根目录配置文件")
	}
	// .plainship 状态目录.
	if !dirExists(filepath.Join(root, ".plainship", "state")) {
		t.Error("缺少 .plainship/state")
	}
	if !dirExists(filepath.Join(root, ".plainship", "cache")) {
		t.Error("缺少 .plainship/cache")
	}
	if !dirExists(filepath.Join(root, ".plainship", "manifests")) {
		t.Error("缺少 .plainship/manifests")
	}
	if !dirExists(filepath.Join(root, ".plainship", "builds")) {
		t.Error("缺少 .plainship/builds")
	}
	// 默认主题可加载.
	if s.Config.Theme.Name != "default" {
		t.Errorf("主题 = %q", s.Config.Theme.Name)
	}
}

func TestCreate_RejectsExistingSpace(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(root); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(root); err == nil {
		t.Error("重复创建应报错")
	}
}

func TestCreate_GitignoreContent(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{".plainship/", "build/"} {
		if !strings.Contains(content, want) {
			t.Errorf(".gitignore 缺少 %s", want)
		}
	}
	if strings.Contains(content, ".git") {
		t.Error(".gitignore 不应包含 .git 相关内容")
	}
}

func TestLoad_AndLoadDocs(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(root); err != nil {
		t.Fatal(err)
	}
	s, err := Load(root)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if s.Root == "" {
		t.Error("Root 为空")
	}
	if !dirExists(s.DocsDir()) {
		t.Error("DocsDir 不存在")
	}
	if s.BuildDir() != filepath.Join(root, "build") {
		t.Errorf("BuildDir = %s", s.BuildDir())
	}
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
