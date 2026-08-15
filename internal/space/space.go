// Package space 管理 Plainship 单个 Space 的创建与加载.
//
// Space 是 Plainship 的基本单位, 一个目录即一个 Space.
package space

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/emanyzwww/plainship/internal/config"
	"github.com/emanyzwww/plainship/internal/git"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/layout"
	"github.com/emanyzwww/plainship/internal/state"
	"github.com/emanyzwww/plainship/internal/theme"
	"github.com/emanyzwww/plainship/internal/ui"
)

// Space 表示一个 Plainship 的 Space.
type Space struct {
	Root         string         // Root 是 Space 根目录.
	GitRoot      string         // GitRoot 是 Git 仓库根目录, 可能位于 Root 的上级.
	GitAvailable bool           // GitAvailable 表示 Git 是否可用.
	Config       *config.Config // Config 是加载后的生效配置, cfg 统一模型.
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

// Create 在指定目录创建新的 Space.
//
// u 是输出入口, Git 缺失等警告经它输出到 stderr; nil 表示静默.
func Create(root string, u ui.UI) (*Space, error) {
	if u == nil {
		u = ui.Discard
	}

	// 1. 解析 Space 根目录为绝对路径.
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	// 2. 检查目录是否已是 Space.
	if config.IsSpaceRoot(abs) {
		return nil, i18n.Errorf(i18n.SpaceAlreadySpace, abs)
	}

	// 3. 创建基础目录.
	for _, d := range []string{layout.DocsDir, layout.ThemesDir} {
		if err := os.MkdirAll(filepath.Join(abs, d), 0o755); err != nil {
			return nil, i18n.Errorf(i18n.SpaceMkdirFail, err)
		}
	}

	// 4. 写入默认配置到根目录 plainship.yaml.
	c := config.Default()
	c.SetSpaceRoot(abs)
	if _, err := config.Save(c, config.SaveSpace); err != nil {
		return nil, i18n.Errorf(i18n.SpaceSaveConfigFail, err)
	}
	// 5. 生成默认主题.
	if err := theme.CopyTo(filepath.Join(abs, layout.ThemesDir, "default")); err != nil {
		return nil, i18n.Errorf(i18n.SpaceThemeFail, err)
	}
	// 6. 初始化 .plainship 状态目录.
	if err := state.EnsureDirs(abs); err != nil {
		return nil, i18n.Errorf(i18n.SpaceStateFail, err)
	}
	s := &Space{Root: abs, Config: c, GitAvailable: git.Available()}

	// 7. Git 感知.
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
		u.Warn(i18n.T(i18n.SpaceGitMissing1))
		u.Warn(i18n.T(i18n.SpaceGitMissing2))
		u.Warn(i18n.T(i18n.SpaceGitMissing3))
	}
	// 8. 生成 .gitignore.
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
	c, _, err := config.Load(abs, nil)
	if err != nil {
		return nil, err
	}
	c.SetSpaceRoot(abs)
	s := &Space{Root: abs, Config: c, GitAvailable: git.Available()}
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
//
// 规则: 永远忽略 `.plainship/` 内部状态与 `build/` 构建产物, 均可由源码重建.
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
