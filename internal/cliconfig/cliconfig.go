// Package cliconfig 管理 CLI 行为配置 (与站点配置 plainship.yaml 分离).
//
// 配置文件只描述 CLI 程序自身的行为 (如工具语言 lang), 与站点内容无关:
//   - 全局: ~/.plainship/config.yaml
//   - 项目: <space>/.plainship/config.yaml (项目覆盖全局)
//
// 优先级: --lang 参数 > PLAINSHIP_LANG 环境变量 > 项目配置 > 全局配置 > 默认.
package cliconfig

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FileName 是 CLI 行为配置文件.
const FileName = "config.yaml"

// Config 是 CLI 行为配置.
type Config struct {
	// Lang 是 CLI 工具语言 (zh / en), 空表示未设置 (使用上层配置或默认).
	Lang string `yaml:"lang,omitempty"`
}

// GlobalPath 返回全局配置文件路径 (~/.plainship/config.yaml).
func GlobalPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".plainship", FileName), nil
}

// ProjectPath 返回项目配置文件路径 (<space>/.plainship/config.yaml).
func ProjectPath(spaceRoot string) string {
	return filepath.Join(spaceRoot, ".plainship", FileName)
}

// LoadGlobal 读取全局配置; 文件不存在时返回空配置.
func LoadGlobal() (Config, error) {
	p, err := GlobalPath()
	if err != nil {
		return Config{}, err
	}
	return load(p)
}

// LoadProject 读取项目配置; 文件不存在时返回空配置.
func LoadProject(spaceRoot string) (Config, error) {
	return load(ProjectPath(spaceRoot))
}

// LoadEffective 计算生效配置: 项目覆盖全局.
func LoadEffective(spaceRoot string) Config {
	cfg := Config{}
	if c, err := LoadGlobal(); err == nil {
		cfg = merge(cfg, c)
	}
	if c, err := LoadProject(spaceRoot); err == nil {
		cfg = merge(cfg, c)
	}
	return cfg
}

// SaveGlobal 写入全局配置 (0600).
func SaveGlobal(cfg Config) error {
	p, err := GlobalPath()
	if err != nil {
		return err
	}
	return save(p, cfg)
}

// SaveProject 写入项目配置 (0600).
func SaveProject(spaceRoot string, cfg Config) error {
	return save(ProjectPath(spaceRoot), cfg)
}

func merge(base, over Config) Config {
	if over.Lang != "" {
		base.Lang = over.Lang
	}
	return base
}

func load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
