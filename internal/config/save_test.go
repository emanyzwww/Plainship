package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSave_Global(t *testing.T) {
	home := isolateHome(t)
	cfg := Default()
	if err := cfg.GlobalClient.Lang.Set("zh"); err != nil {
		t.Fatal(err)
	}
	path, err := Save(cfg, SaveGlobal)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".plainship", "config.yaml")
	if path != want {
		t.Errorf("路径 = %s, 期望 %s", path, want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "lang: zh") {
		t.Errorf("文件内容缺少 lang: %s", data)
	}
	// 读回验证.
	cfg2, _, err := Load("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Lang() != "zh" {
		t.Errorf("读回 lang = %q", cfg2.Lang())
	}
}

func TestSave_Project(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	cfg := Default()
	cfg.SetSpaceRoot(root)
	if err := cfg.SpaceClient.Color.Set(false); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(cfg, SaveProject); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".plainship", "config.yaml")
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "color: false") {
		t.Errorf("文件内容: %s", data)
	}
	// 读回验证.
	cfg2, _, err := Load(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Color() {
		t.Error("读回 color 应为 false")
	}
}

func TestSave_Space(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	cfg := Default()
	cfg.SetSpaceRoot(root)
	if err := cfg.SpaceSite.SiteTitle.Set("我的网站"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SpaceSite.MarkdownUnsafe.Set(true); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(cfg, SaveSpace); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "plainship.yaml")
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "title: 我的网站") {
		t.Errorf("缺少 title: %s", text)
	}
	if !strings.Contains(text, "unsafe: true") {
		t.Errorf("缺少 unsafe: %s", text)
	}
	// 读回验证.
	cfg2, _, err := Load(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.SpaceSite.SiteTitle.Get() != "我的网站" {
		t.Errorf("读回 title = %q", cfg2.SpaceSite.SiteTitle.Get())
	}
	if !cfg2.SpaceSite.MarkdownUnsafe.Get() {
		t.Error("读回 unsafe 应为 true")
	}
}

func TestSave_TokenInProjectConfig(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	cfg := Default()
	cfg.SetSpaceRoot(root)
	if err := cfg.SpaceClient.ServerToken.Set("ps_secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(cfg, SaveProject); err != nil {
		t.Fatal(err)
	}
	// 令牌写入 `.plainship/config.yaml`.
	data, err := os.ReadFile(filepath.Join(root, ".plainship", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "server:") || !strings.Contains(string(data), "token: ps_secret") {
		t.Errorf("令牌应写入项目配置文件: %s", data)
	}
	// 读回验证.
	cfg2, _, err := Load(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.ServerToken() != "ps_secret" {
		t.Errorf("读回 token = %q", cfg2.ServerToken())
	}
}

func TestSave_TokenNotInSpaceConfig(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	cfg := Default()
	cfg.SetSpaceRoot(root)
	if err := cfg.SpaceSite.ServerURL.Set("http://localhost:9090"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SpaceClient.ServerToken.Set("ps_secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(cfg, SaveSpace); err != nil {
		t.Fatal(err)
	}
	// plainship.yaml 不应包含令牌.
	data, err := os.ReadFile(filepath.Join(root, "plainship.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "token") {
		t.Errorf("plainship.yaml 不应包含令牌: %s", data)
	}
}

func TestSave_UnknownKeysRemoved(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	// 先写一个含多余键的文件.
	writeFile(t, filepath.Join(root, "plainship.yaml"), "site:\n  title: 旧标题\nstale:\n  key: 残留\n")
	cfg := Default()
	cfg.SetSpaceRoot(root)
	if err := cfg.SpaceSite.SiteTitle.Set("新标题"); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(cfg, SaveSpace); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "plainship.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "title: 新标题") {
		t.Errorf("缺少新 title: %s", text)
	}
	if strings.Contains(text, "stale") {
		t.Errorf("未知键 stale 应被移除: %s", text)
	}
}

func TestSave_DefaultsWritten(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	// 全空 cfg: Save 写出完整默认配置.
	cfg := Default()
	cfg.SetSpaceRoot(root)
	if _, err := Save(cfg, SaveSpace); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "plainship.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "title: 我的文档") || !strings.Contains(text, "output: build") {
		t.Errorf("应写出默认配置: %s", text)
	}
}

func TestSave_SpaceYAMLRoundTrip(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "plainship.yaml"), "site:\n  title: A\n  url: https://a.com\nmarkdown:\n  unsafe: true\nbuild:\n  output: dist\n")
	cfg, _, err := Load(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SpaceSite.SiteTitle.Get() != "A" || cfg.SpaceSite.SiteURL.Get() != "https://a.com" {
		t.Errorf("site 读回不对: %q %q", cfg.SpaceSite.SiteTitle.Get(), cfg.SpaceSite.SiteURL.Get())
	}
	if !cfg.SpaceSite.MarkdownUnsafe.Get() {
		t.Error("unsafe 读回应为 true")
	}
	if cfg.SpaceSite.BuildOutput.Get() != "dist" {
		t.Errorf("output = %q", cfg.SpaceSite.BuildOutput.Get())
	}
}

// TestSave_SkipDefaults 全局/空间级工具配置跳过默认值, 默认值不固化进文件.
func TestSave_SkipDefaults(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	cfg := Default()
	cfg.SetSpaceRoot(root)
	// 只设置非默认的 color, lang 保持默认 en.
	if err := cfg.SpaceClient.Color.Set(false); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(cfg, SaveProject); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".plainship", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "color: false") {
		t.Errorf("非默认值应写入: %s", text)
	}
	if strings.Contains(text, "lang: en") {
		t.Errorf("默认值 lang 不应写入文件: %s", text)
	}
	// 读回: lang 保持未设置, 生效值仍为默认 en.
	cfg2, _, err := Load(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.SpaceClient.Lang.HasValue() {
		t.Error("默认值不应使 lang 变为已设置")
	}
	if cfg2.Lang() != "en" {
		t.Errorf("lang 生效值 = %q", cfg2.Lang())
	}
}

// TestSave_SpaceRootRequired 写空间层文件必须已设置 Space 根目录.
func TestSave_SpaceRootRequired(t *testing.T) {
	isolateHome(t)
	cfg := Default()
	if _, err := Save(cfg, SaveProject); err == nil {
		t.Error("SaveProject 未设置 spaceRoot 应报错")
	}
	if _, err := Save(cfg, SaveSpace); err == nil {
		t.Error("SaveSpace 未设置 spaceRoot 应报错")
	}
	// 全局层不需要 spaceRoot.
	if _, err := Save(cfg, SaveGlobal); err != nil {
		t.Errorf("SaveGlobal 不应要求 spaceRoot: %v", err)
	}
}

// TestSave_SkipDefaultsKeepsGlobalVisible 空间未设置的键不固化, 全局修改仍可见.
func TestSave_SkipDefaultsKeepsGlobalVisible(t *testing.T) {
	home := isolateHome(t)
	root := t.TempDir()
	// 全局 lang=zh.
	writeFile(t, filepath.Join(home, ".plainship", "config.yaml"), "lang: zh\n")
	// 空间保存过 color false, 但 lang 未设置.
	cfg := Default()
	cfg.SetSpaceRoot(root)
	if err := cfg.SpaceClient.Color.Set(false); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(cfg, SaveProject); err != nil {
		t.Fatal(err)
	}
	// lang 生效值应回退全局 zh, 空间文件未固化 lang.
	cfg2, _, err := Load(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Lang() != "zh" {
		t.Errorf("空间未设置 lang 时应跟随全局, 实际 %q", cfg2.Lang())
	}
}
