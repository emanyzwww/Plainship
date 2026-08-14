// Package space 管理 Plainship 单个 Space 的创建与加载.
// Space 是 Plainship 的基本单位, 一个目录即一个 Space.
package space

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emanyzwww/plainship/internal/config"
	"github.com/emanyzwww/plainship/internal/git"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/layout"
	"github.com/emanyzwww/plainship/internal/state"
	"github.com/emanyzwww/plainship/internal/theme"
)

// Space 表示一个 Plainship 的 Space
type Space struct {
	Root    string
	GitRoot string
	// GitAvailable 表示 Git 是否可用
	GitAvailable bool
	// Config 是加载后的配置
	Config config.Config
}

// DocsDir 返回 docs 目录路径.
func (s *Space) DocsDir() string {
	return filepath.Join(s.Root, layout.DocsDir)
}

// ThemesDir 返回 themes 目录路径.
func (s *Space) ThemesDir() string {
	return filepath.Join(s.Root, layout.ThemesDir)
}

// BuildDir 返回构建输出目录路径.
func (s *Space) BuildDir() string {
	return filepath.Join(s.Root, layout.BuildDir)
}

// Create 在指定目录创建新的 Space
func Create(root string) (*Space, error) {
	// 拿到 space 的 root path
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	// 检查目录
	if config.IsSpaceRoot(abs) {
		return nil, i18n.Errorf(i18n.SpaceAlreadySpace, abs)
	}

	// 创建基础目录
	// 配置直接写在根目录 plainship.yaml
	for _, d := range []string{layout.DocsDir, layout.ThemesDir} {
		if err := os.MkdirAll(filepath.Join(abs, d), 0o755); err != nil {
			return nil, i18n.Errorf(i18n.SpaceMkdirFail, err)
		}
	}

	// 写入默认配置
	cfg := config.Default()
	if err := config.Save(abs, cfg); err != nil {
		return nil, i18n.Errorf(i18n.SpaceSaveConfigFail, err)
	}
	// 4. 生成默认主题.
	if err := theme.CopyTo(filepath.Join(abs, layout.ThemesDir, "default")); err != nil {
		return nil, i18n.Errorf(i18n.SpaceThemeFail, err)
	}
	// 5. 初始化 .plainship 状态目录.
	if err := state.EnsureDirs(abs); err != nil {
		return nil, i18n.Errorf(i18n.SpaceStateFail, err)
	}
	s := &Space{Root: abs, Config: cfg, GitAvailable: git.Available()}

	// 6. Git 感知.
	if s.GitAvailable {
		if git.IsRepo(abs) {
			s.GitRoot, _ = git.Root(abs)
		} else {
			// 默认直接初始化 Git, 不提供选项.
			if err := git.Init(abs); err != nil {
				return nil, err
			}
			s.GitRoot = abs
		}
	} else {
		fmt.Fprintln(os.Stderr, i18n.T(i18n.SpaceGitMissing1))
		fmt.Fprintln(os.Stderr, i18n.T(i18n.SpaceGitMissing2))
		fmt.Fprintln(os.Stderr, i18n.T(i18n.SpaceGitMissing3))
	}
	// 7. 生成 .gitignore.
	if err := writeGitignore(abs); err != nil {
		return nil, err
	}
	return s, nil
}

// Load 加载已有 Space.
func Load(root string) (*Space, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if !config.IsSpaceRoot(abs) {
		return nil, i18n.Errorf(i18n.SpaceNotFound, abs)
	}
	cfg, err := config.Load(abs)
	if err != nil {
		return nil, err
	}
	s := &Space{Root: abs, Config: cfg, GitAvailable: git.Available()}
	if s.GitAvailable && git.IsRepo(abs) {
		s.GitRoot, _ = git.Root(abs)
	}
	return s, nil
}

// FindFrom 从当前目录向上查找 Space 并加载.
func FindFrom(dir string) (*Space, error) {
	root, err := config.FindRoot(dir)
	if err != nil {
		return nil, err
	}
	return Load(root)
}

// writeGitignore 生成或更新 .gitignore.
// 规则:
//   - 永远忽略 .plainship/ 内部状态与 build/ 构建产物 (均可由源码重建).
func writeGitignore(root string) error {
	path := filepath.Join(root, ".gitignore")
	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}
	var lines []string
	if existing != "" {
		lines = append(lines, strings.TrimRight(existing, "\n"))
	}
	appendLine := func(s string) {
		for _, l := range lines {
			if strings.TrimSpace(l) == s {
				return
			}
		}
		lines = append(lines, s)
	}
	appendLine("# Plainship runtime state (regenerable)")
	appendLine(layout.StateDir + "/")
	appendLine("# Build output stays out of Git (reproducible from docs + themes + config)")
	appendLine(layout.BuildDir + "/")
	appendLine(".DS_Store")
	appendLine("*.log")
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}

// CheckGit 输出 Git 集成状态摘要.
func (s *Space) CheckGit() (branch string, clean bool, hasRepo bool) {
	if !s.GitAvailable {
		return "", false, false
	}
	if s.GitRoot == "" {
		return "", false, false
	}
	return git.Branch(s.Root), git.Clean(s.Root), true
}
