package scanner

import "github.com/emanyzwww/papership-client/model/space"

// Kind 表示扫描识别出的文件类型分类.
type Kind int

const (
	KindDoc     Kind = 0 // KindDoc 文档: docs 目录下的 Markdown 文件.
	KindAsset   Kind = 1 // KindAsset 静态资源: 站点构建时需要原样拷贝的非文档文件.
	KindTheme   Kind = 2 // KindTheme 主题: themes 目录下的一级条目.
	KindUnknown Kind = 3 // KindUnknown 暂未分类的文件.
)

func (k Kind) String() string {
	switch k {
	case KindDoc:
		return "doc"
	case KindTheme:
		return "theme"
	case KindAsset:
		return "asset"
	default:
		return "unknown"
	}
}

// DocEntry 是 docs 目录下的一篇文档在扫描索引中的记录.
type DocEntry struct {
	AbsPath string // AbsPath 文件绝对路径.
	RelPath string // RelPath 相对 Space 根目录的路径.
	Dir     string // Dir 相对 docs 根目录的目录部分; 顶层文档为空.
	Base    string // Base 文件名(含扩展名).
	Stem    string // Stem 文件名(不含扩展名).
	Ext     string // Ext 扩展名, 小写, 含点, 如 ".md".
	Size    int64  // Size 文件字节数.
	ModTime int64  // ModTime 修改时间, 供增量扫描比对.
}

// ThemeEntry 是 themes 目录下的一个主题条目.
type ThemeEntry struct {
	Name    string // Name 主题名: 即 themes 下的一级目录名.
	AbsPath string // AbsPath 主题目录绝对路径.
	RelPath string // RelPath 相对 Space 根目录的路径.
}

// AssetEntry 是静态资源的扫描索引记录.
type AssetEntry struct {
	AbsPath string // AbsPath 文件绝对路径.
	RelPath string // RelPath 相对 Space 根目录的路径.
	Size    int64  // Size 文件字节数.
	ModTime int64  // ModTime 修改时间.
}

// Problem 记录扫描过程中的非致命问题.
type Problem struct {
	Severity string // Severity: "warning" 或 "error".
	Path     string // Path 关联的路径.
	Message  string // Message 人类可读的描述.
}

// 常用严重级别.
const (
	SeverityWarning = "warning"
	SeverityError   = "error"
)

// Result 是一次 Scan 的完整产物: 它既是文件索引, 也是下游的输入.
//
// Space 字段回填了扫描过程中探测到的 GitRoot/GitAvailable,
// 调用方可直接使用扩展后的 Space.
type Result struct {
	Space              *space.Space // Space 本次扫描的 Space.
	Docs               []DocEntry   // Docs 文档索引, 按 RelPath 排序.
	Themes             []ThemeEntry // Themes 主题清单, 按 Name 排序.
	Assets             []AssetEntry // Assets 静态资源索引, 按 RelPath 排序.
	Problems           []Problem    // Problems 扫描中收集的问题.
	ScannedAt          int64        // ScannedAt 扫描开始时间.
	ConfigPresent      bool         // ConfigPresent 根目录 papership.yaml 是否存在.
	LocalConfigPresent bool         // LocalConfigPresent .papership/config.yaml 是否存在.
}

// DocCount 返回文档数量.
func (r *Result) DocCount() int { return len(r.Docs) }

// ThemeCount 返回主题数量.
func (r *Result) ThemeCount() int { return len(r.Themes) }

// AssetCount 返回静态资源数量.
func (r *Result) AssetCount() int { return len(r.Assets) }
