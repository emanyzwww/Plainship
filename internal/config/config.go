// Package config 负责加载与保存 Plainship Space 的配置.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/emanyzwww/Plainship/internal/hash"
	"github.com/emanyzwww/Plainship/internal/i18n"
	"github.com/emanyzwww/Plainship/internal/layout"
	"gopkg.in/yaml.v3"
)

// FileName 是配置文件名
const FileName = layout.ConfigFile

// Dir 返回配置所在目录
func Dir(spaceRoot string) string {
	return spaceRoot
}

// Path 返回配置文件路径
func Path(spaceRoot string) string {
	return filepath.Join(spaceRoot, FileName)
}

// Config 是完整的 Plainship 配置
type Config struct {
	Site     SiteConfig     `yaml:"site"`
	Build    BuildConfig    `yaml:"build"`
	Theme    ThemeConfig    `yaml:"theme"`
	List     ListConfig     `yaml:"list"`
	Markdown MarkdownConfig `yaml:"markdown"`
	Server   ServerConfig   `yaml:"server"`
}

// SiteConfig 是站点级配置
type SiteConfig struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	URL         string `yaml:"url"`
	Language    string `yaml:"language"`
	SiteID      string `yaml:"siteId"`
}

// BuildConfig 是构建配置
type BuildConfig struct {
	Output string `yaml:"output"`
}

// ThemeConfig 是主题配置
type ThemeConfig struct {
	Name string `yaml:"name"`
}

// ListConfig 是列表页排序配置
type ListConfig struct {
	Sort string `yaml:"sort"`
}

// MarkdownConfig 是 Markdown 渲染配置.
type MarkdownConfig struct {
	// Unsafe 为 true 时允许正文中的原始 HTML 直接输出 (不转义).
	// 默认 false: 原始 HTML 会被转义为文本, 防止发布站点 XSS.
	Unsafe bool `yaml:"unsafe"`
}

// ServerConfig 是远程服务器配置
type ServerConfig struct {
	URL   string `yaml:"url"`
	Site  string `yaml:"site"`
	Token string `yaml:"token"`
}

// Default 返回默认配置
func Default() Config {
	return Config{
		Site: SiteConfig{
			Title:       "我的文档",
			Description: "Plainship 文档",
			URL:         "https://example.com",
			Language:    "en",
			SiteID:      "my-docs",
		},
		Build: BuildConfig{
			Output: layout.BuildDir,
		},
		Theme: ThemeConfig{
			Name: "default",
		},
		List: ListConfig{
			Sort: "date-desc",
		},
		Markdown: MarkdownConfig{
			Unsafe: false,
		},
		Server: ServerConfig{
			URL:  "",
			Site: "my-docs",
		},
	}
}

// Load 加载配置
// 读取根目录 plainship.yaml
// 如果文件不存在, 返回默认配置而不报错
func Load(spaceRoot string) (Config, error) {
	// 默认配置
	cfg := Default()
	// 默认 path
	path := Path(spaceRoot)
	// 配置文件是否存在
	exists := fileExists(path)

	if !exists {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, i18n.Errorf(i18n.ConfigParseFail, err)
	}

	// 令牌从 .plainship/server.token 读取 (兼容旧版 yaml 内联令牌).
	if cfg.Server.Token == "" {
		cfg.Server.Token = loadToken(spaceRoot)
	}

	return cfg, nil
}

// Save 写入配置
// 写入根目录 plainship.yaml.
// 访问令牌不写入 yaml (避免随 config 类别提交进 Git 历史),
// 而是单独保存到 .plainship/server.token (0600, 已被 .gitignore 忽略).
func Save(spaceRoot string, cfg Config) error {
	token := cfg.Server.Token
	cfg.Server.Token = ""

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	if err := os.WriteFile(Path(spaceRoot), data, 0o644); err != nil {
		return err
	}

	if token != "" {
		return saveToken(spaceRoot, token)
	}
	return nil
}

// tokenFilePath 返回 Space 内令牌文件路径 (.plainship/server.token).
// 该目录被 .gitignore 忽略, 令牌不会进入 Git.
func tokenFilePath(spaceRoot string) string {
	return filepath.Join(spaceRoot, layout.StateDir, "server.token")
}

// saveToken 将令牌写入 .plainship/server.token, 权限 0600.
func saveToken(spaceRoot, token string) error {
	if err := os.MkdirAll(filepath.Dir(tokenFilePath(spaceRoot)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(tokenFilePath(spaceRoot), []byte(token+"\n"), 0o600)
}

// loadToken 读取 Space 内令牌文件, 不存在时返回空字符串.
func loadToken(spaceRoot string) string {
	data, err := os.ReadFile(tokenFilePath(spaceRoot))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Hash 计算配置的规范化哈希, 用于变化检测
// 令牌不属于内容指纹: connect 更换令牌不应使构建过期.
func (cfg Config) Hash() (string, error) {
	cfg.Server.Token = ""
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}

	return hash.Bytes(data), nil
}

// FileHash 读取配置文件并计算哈希
// 文件不存在时返回空字符串且不报错
func FileHash(spaceRoot string) (string, error) {
	path := Path(spaceRoot)
	if !fileExists(path) {
		return "", nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return hash.Bytes(data), nil
}

// IsSpaceRoot 判断目录是否为 Plainship Space 根目录
func IsSpaceRoot(dir string) bool {
	return dirExists(dir) && fileExists(filepath.Join(dir, layout.ConfigFile))
}

// FindRoot 从 dir 向上逐级查找 Space 根目录
func FindRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	cur := abs

	for {
		if IsSpaceRoot(cur) {
			return cur, nil
		}

		parent := filepath.Dir(cur)

		if parent == cur {
			return "", i18n.Errorf(i18n.ConfigNotFound)
		}

		cur = parent
	}
}

// dirExists 判断目录是否存在
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fileExists 判断文件是否存在
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
