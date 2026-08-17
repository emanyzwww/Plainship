package space

import (
	"io/fs"
	"os"
	"path/filepath"
)

// Space 表示一个 PaperShip 内容根: 一个目录即一个 Space.
//
// 它是 scanner 的输入源, 一次 Space 构建产出一个 Config.
type Space struct {
	Root         string      // Root 是 Space 根目录.
	Layout       Layout      // Layout 是当前生效的目录布局.
	Config       Config      // Config 是站点级配置.
	GitRoot      string      // GitRoot 是 Git 仓库根目录; 可能与 Root 相同, 也可能位于 Root 上级; 空表示尚未检测.
	LocalConfig  LocalConfig // LocalConfig 是本地私有配置.
	GitAvailable bool        // GitAvailable 表示本机 Git 是否可用.
}

// Name 返回 Space 展示名: 优先取 siteId, 否则取根目录名.
func (s *Space) Name() string {
	siteID := s.Config.SiteID
	if siteID != "" {
		return siteID
	}
	return filepath.Base(s.Root)
}

// DocsDir 返回文档目录路径.
func (s *Space) DocsDir() string {
	return filepath.Join(s.Root, s.Layout.Docs)
}

// ThemesDir 返回主题目录路径.
func (s *Space) ThemesDir() string {
	return filepath.Join(s.Root, s.Layout.Themes)
}

// BuildDir 返回构建输出目录路径.
func (s *Space) BuildDir() string {
	dir := s.Layout.Build
	if !filepath.IsAbs(dir) {
		return filepath.Join(s.Root, dir)
	}
	return dir
}

// StateDir 返回内部状态目录路径.
func (s *Space) StateDir() string {
	return filepath.Join(s.Root, s.Layout.State)
}

// ConfigPath 返回空间配置文件路径.
func (s *Space) ConfigPath() string {
	return filepath.Join(s.Root, s.Layout.Config)
}

// IsSpaceRoot 判断 dir 是否是一个 Space 根目录.
func IsSpaceRoot(dir string) bool {
	info, err := osStat(filepath.Join(dir, ConfigFile))
	return err == nil && !info.IsDir()
}

// osStat 是 os.Stat 的可测试替身; core 层可按需注入.
var osStat = func(name string) (fs.FileInfo, error) { return os.Stat(name) }
