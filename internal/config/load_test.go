package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateHome 隔离用户主目录, Windows 用 USERPROFILE, 其他平台用 HOME.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// writeFile 写入测试配置文件.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_Defaults(t *testing.T) {
	isolateHome(t)
	cfg, warns, err := Load("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Errorf("无来源时应无警告, 实际 %v", warns)
	}
	if cfg.Lang() != "en" {
		t.Errorf("默认 lang = %q, 期望 en", cfg.Lang())
	}
	if !cfg.Color() {
		t.Error("默认 color 应为 true")
	}
	if cfg.SpaceSite.SiteTitle.Get() != "我的文档" {
		t.Errorf("默认 title = %q", cfg.SpaceSite.SiteTitle.Get())
	}
	if cfg.SpaceSite.BuildOutput.Get() != "build" {
		t.Errorf("默认 output = %q", cfg.SpaceSite.BuildOutput.Get())
	}
}

func TestLoad_PriorityFlagOverEnv(t *testing.T) {
	isolateHome(t)
	t.Setenv("PLAINSHIP_LANG", "zh")
	cfg, _, err := Load("", map[string]string{"lang": "en"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Lang() != "en" {
		t.Errorf("flag 应覆盖 env, 实际 %q", cfg.Lang())
	}
}

func TestLoad_PriorityEnvOverGlobal(t *testing.T) {
	home := isolateHome(t)
	writeFile(t, filepath.Join(home, ".plainship", "config.yaml"), "lang: zh\n")
	cfg, _, err := Load("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Lang() != "zh" {
		t.Errorf("全局配置应生效, 实际 %q", cfg.Lang())
	}
	t.Setenv("PLAINSHIP_LANG", "en")
	cfg, _, err = Load("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Lang() != "en" {
		t.Errorf("env 应覆盖全局, 实际 %q", cfg.Lang())
	}
}

func TestLoad_SpaceOverGlobal(t *testing.T) {
	root := t.TempDir()
	// 全局 lang=zh.

	home := isolateHome(t)
	writeFile(t, filepath.Join(home, ".plainship", "config.yaml"), "lang: zh\n")
	// 空间级客户端配置 lang=en.
	writeFile(t, filepath.Join(root, ".plainship", "config.yaml"), "lang: en\n")
	// 空间网站配置.
	writeFile(t, filepath.Join(root, "plainship.yaml"), "site:\n  language: zh-CN\n  title: 自定义标题\n")
	cfg, _, err := Load(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Lang() != "en" {
		t.Errorf("空间配置应覆盖全局, 实际 %q", cfg.Lang())
	}
	if cfg.SpaceSite.SiteLanguage.Get() != "zh-CN" {
		t.Errorf("空间 language 应生效, 实际 %q", cfg.SpaceSite.SiteLanguage.Get())
	}
	if cfg.SpaceSite.SiteTitle.Get() != "自定义标题" {
		t.Errorf("空间 title 应生效, 实际 %q", cfg.SpaceSite.SiteTitle.Get())
	}
}

func TestLoad_InvalidFallsBackToDefault(t *testing.T) {
	isolateHome(t)
	// env 提供非法 lang.
	t.Setenv("PLAINSHIP_LANG", "fr")
	cfg, warns, err := Load("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Lang() != "en" {
		t.Errorf("非法值应回退默认 en, 实际 %q", cfg.Lang())
	}
	if len(warns) != 1 {
		t.Fatalf("应有 1 条警告, 实际 %d", len(warns))
	}
	w := warns[0]
	if w.Key != "lang" || w.Given != "fr" || w.Fallback != "en" {
		t.Errorf("警告内容不对: %+v", w)
	}
}

func TestLoad_InvalidSpaceValue(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	// 空间配置 markdown.unsafe 为非法布尔.
	writeFile(t, filepath.Join(root, "plainship.yaml"), "markdown:\n  unsafe: 不是布尔\n")
	cfg, warns, err := Load(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SpaceSite.MarkdownUnsafe.Get() {
		t.Error("非法布尔应回退默认 false")
	}
	found := false
	for _, w := range warns {
		if w.Key == "markdown.unsafe" {
			found = true
		}
	}
	if !found {
		t.Errorf("应有 markdown.unsafe 警告, 实际 %+v", warns)
	}
}

func TestLoad_InvalidRuntimeFallsBackToDefault(t *testing.T) {
	home := isolateHome(t)
	writeFile(t, filepath.Join(home, ".plainship", "config.yaml"), "lang: zh\n")
	// env 传非法值: 回退默认, 不尝试更低层.
	t.Setenv("PLAINSHIP_LANG", "fr")
	cfg, warns, err := Load("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Lang() != "en" {
		t.Errorf("非法运行时值应回退默认 en, 实际 %q", cfg.Lang())
	}
	if len(warns) != 1 {
		t.Errorf("警告数 = %d, 期望 1", len(warns))
	}
}

func TestLoad_TokenHasNoEnvSource(t *testing.T) {
	isolateHome(t)
	// `server.token` 无环境变量来源.
	t.Setenv("PLAINSHIP_TOKEN", "secret")
	cfg, _, err := Load("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerToken() != "" {
		t.Errorf("token 不应来自环境变量, 实际 %q", cfg.ServerToken())
	}
}

func TestLoad_CorruptedFileFails(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "plainship.yaml"), "site: [broken\n")
	if _, _, err := Load(root, nil); err == nil {
		t.Error("损坏的 plainship.yaml 应报错")
	} else if !strings.Contains(err.Error(), "plainship.yaml") {
		t.Errorf("错误信息应包含文件路径: %v", err)
	}
}

func TestLoad_UnknownKeysIgnored(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	// 未来版本残留的未知键应被忽略, 不影响其他配置.
	writeFile(t, filepath.Join(root, "plainship.yaml"), "site:\n  title: 好标题\nunknown:\n  key: value\n")
	cfg, _, err := Load(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SpaceSite.SiteTitle.Get() != "好标题" {
		t.Errorf("未知键不应干扰已知键: %q", cfg.SpaceSite.SiteTitle.Get())
	}
}

func TestLoad_NoSpaceRootSkipsSpaceLayer(t *testing.T) {
	isolateHome(t)
	empty := t.TempDir()
	cfg, _, err := Load(empty, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SpaceSite.SiteTitle.Get() != "我的文档" {
		t.Errorf("非 Space 应保持默认: %q", cfg.SpaceSite.SiteTitle.Get())
	}
}

// TestLoad_EnvOnlyClientKeys 环境变量只作用于客户端工具键, 不影响空间网站配置.
func TestLoad_EnvOnlyClientKeys(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "plainship.yaml"), "site:\n  title: 原标题\n")
	t.Setenv("PLAINSHIP_SITE_TITLE", "环境变量标题")
	t.Setenv("PLAINSHIP_COLOR", "false")
	cfg, _, err := Load(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SpaceSite.SiteTitle.Get() != "原标题" {
		t.Errorf("环境变量不应作用于站点配置: %q", cfg.SpaceSite.SiteTitle.Get())
	}
	if cfg.Color() {
		t.Error("PLAINSHIP_COLOR=false 应生效")
	}
}
