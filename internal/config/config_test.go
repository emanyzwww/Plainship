package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/emanyzwww/plainship/internal/layout"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Site.Title != "我的文档" {
		t.Errorf("默认标题 = %q", cfg.Site.Title)
	}
	if cfg.Build.Output != "build" {
		t.Errorf("默认输出 = %q", cfg.Build.Output)
	}
	if cfg.Theme.Name != "default" {
		t.Errorf("默认主题 = %q", cfg.Theme.Name)
	}
	if cfg.Markdown.Unsafe {
		t.Error("默认 markdown.unsafe 应为 false")
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	root := t.TempDir()
	cfg := Default()
	cfg.Site.Title = "自定义标题"
	cfg.Server.URL = "http://localhost:9090"
	cfg.Server.Token = "secret"
	if err := Save(root, cfg); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if loaded.Site.Title != "自定义标题" {
		t.Errorf("标题 = %q", loaded.Site.Title)
	}
	if loaded.Server.URL != "http://localhost:9090" {
		t.Errorf("服务器 = %q", loaded.Server.URL)
	}
	if loaded.Server.Token != "secret" {
		t.Error("Token 往返失败")
	}
}

func TestLoad_MissingFileReturnsDefault(t *testing.T) {
	root := t.TempDir()
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if cfg.Site.Title != "我的文档" {
		t.Error("应返回默认配置")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte("site: [未闭合"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Error("无效 YAML 应报错")
	}
}

// TestSaveLoad_TokenNotInYAML 验证令牌不写入 plainship.yaml,
// 而是保存在 .plainship/server.token (0600), Load 时读回.
func TestSaveLoad_TokenNotInYAML(t *testing.T) {
	root := t.TempDir()
	cfg := Default()
	cfg.Server.URL = "http://localhost:9090"
	cfg.Server.Token = "ps_secret_token_123"
	if err := Save(root, cfg); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	// yaml 中不得包含令牌.
	data, err := os.ReadFile(filepath.Join(root, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "ps_secret_token_123") {
		t.Error("令牌不应写入 plainship.yaml")
	}
	// 令牌文件存在且权限受限 (Windows 不强制权限位, 跳过).
	tokPath := filepath.Join(root, layout.StateDir, "server.token")
	info, err := os.Stat(tokPath)
	if err != nil {
		t.Fatalf("令牌文件不存在: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("令牌文件权限 = %v, 期望 0600", info.Mode().Perm())
	}
	// Load 读回令牌.
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Server.Token != "ps_secret_token_123" {
		t.Errorf("Load 未读回令牌: %q", loaded.Server.Token)
	}
}

// TestHash_IgnoresToken 验证令牌不影响配置哈希 (connect 换令牌不应使构建过期).
func TestHash_IgnoresToken(t *testing.T) {
	cfg := Default()
	cfg.Server.URL = "http://localhost:9090"
	h1, err := cfg.Hash()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Server.Token = "secret-a"
	h2, err := cfg.Hash()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Server.Token = "secret-b"
	h3, err := cfg.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 || h2 != h3 {
		t.Error("令牌变化不应影响配置哈希")
	}
}

func TestHash_Stable(t *testing.T) {
	cfg := Default()
	h1, err := cfg.Hash()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := cfg.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Error("相同配置的哈希应一致")
	}
	cfg.Site.Title = "改变"
	h3, _ := cfg.Hash()
	if h1 == h3 {
		t.Error("不同配置的哈希不应一致")
	}
}

func TestIsSpaceRoot(t *testing.T) {
	root := t.TempDir()
	if IsSpaceRoot(root) {
		t.Error("空目录不应是 Space")
	}
	if err := os.WriteFile(filepath.Join(root, FileName), []byte("site: {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsSpaceRoot(root) {
		t.Error("有配置文件应识别为 Space")
	}
}

func TestFindRoot_Upward(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte("site: {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	found, err := FindRoot(sub)
	if err != nil {
		t.Fatalf("FindRoot 失败: %v", err)
	}
	abs, _ := filepath.Abs(root)
	if found != abs {
		t.Errorf("found = %s, 期望 %s", found, abs)
	}
}

func TestFindRoot_NotFound(t *testing.T) {
	if _, err := FindRoot(t.TempDir()); err == nil {
		t.Error("无 Space 时应报错")
	}
}
