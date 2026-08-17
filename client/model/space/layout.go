package space

// 目录与文件常量: Space 的标准布局.
const (
	DocsDir    = "docs"           // DocsDir 是文档内容目录名.
	BuildDir   = "build"          // BuildDir 是构建输出目录名.
	StateDir   = ".papership"     // StateDir 是内部状态目录名.
	ThemesDir  = "themes"         // ThemesDir 是主题目录名.
	ConfigFile = "papership.yaml" // ConfigFile 是根目录空间配置文件.
)

// Layout 描述 Space 的目录布局.
//
// 字段是相对路径, 相对 Space 根目录.
type Layout struct {
	Docs   string // 文档目录, 默认 "docs".
	Build  string // 构建输出, 默认 "build".
	State  string // 内部状态, 默认 ".papership".
	Config string // 空间配置文件, 默认 "papership.yaml"..
	Themes string // 主题目录, 默认 "themes".
}

// DefaultLayout 返回标准布局.
func DefaultLayout() Layout {
	return Layout{
		Docs:   DocsDir,
		Themes: ThemesDir,
		Build:  BuildDir,
		State:  StateDir,
		Config: ConfigFile,
	}
}
