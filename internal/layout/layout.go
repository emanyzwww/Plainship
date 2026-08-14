// Package layout 集中定义 Plainship 工作区目录与文件常量.
// 本包处于依赖最底层: 不依赖任何其他 internal 包,
// 保证 config / space / builder 等包可以安全引用这些常量而不产生循环依赖.
package layout

const (
	// DocsDir 是文档目录名.
	DocsDir = "docs"
	// ThemesDir 是主题目录名.
	ThemesDir = "themes"
	// BuildDir 是构建产物目录名.
	BuildDir = "build"
	// StateDir 是内部状态目录名.
	StateDir = ".plainship"
	// ConfigFile 是根目录配置文件.
	ConfigFile = "plainship.yaml"
	// GitignoreFile 是根目录 Git 忽略文件.
	GitignoreFile = ".gitignore"
)
