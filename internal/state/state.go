// Package state 管理 Plainship 的内部运行状态.
// 状态位于 .plainship/ 目录, 与 Git 状态严格分离:
//   - .plainship/state      构建状态, 可重新生成
//   - .plainship/cache      纯缓存, 可随意删除
//   - .plainship/manifests  构建清单, 可恢复
//   - .plainship/builds     原子构建产物
package state

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/emanyzwww/plainship/internal/layout"
)

// Dir 返回 .plainship 目录路径.
func Dir(spaceRoot string) string {
	return filepath.Join(spaceRoot, layout.StateDir)
}

// StateDir 返回 .plainship/state 目录路径.
func StateDir(spaceRoot string) string {
	return filepath.Join(Dir(spaceRoot), "state")
}

// CacheDir 返回 .plainship/cache 目录路径.
func CacheDir(spaceRoot string) string {
	return filepath.Join(Dir(spaceRoot), "cache")
}

// ManifestsDir 返回 .plainship/manifests 目录路径.
func ManifestsDir(spaceRoot string) string {
	return filepath.Join(Dir(spaceRoot), "manifests")
}

// BuildsDir 返回 .plainship/builds 目录路径.
func BuildsDir(spaceRoot string) string {
	return filepath.Join(Dir(spaceRoot), "builds")
}

// BuildDir 返回指定构建 ID 的产物目录.
func BuildDir(spaceRoot, buildID string) string {
	return filepath.Join(BuildsDir(spaceRoot), buildID)
}

// EnsureDirs 创建全部 .plainship 子目录.
func EnsureDirs(spaceRoot string) error {
	for _, d := range []string{StateDir(spaceRoot), CacheDir(spaceRoot), ManifestsDir(spaceRoot), BuildsDir(spaceRoot)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// FileState 记录单个源文件对应的构建状态.
type FileState struct {
	Hash    string `json:"hash"`
	Output  string `json:"output"`
	Route   string `json:"route"`
	Type    string `json:"type"` // page / asset / index
	Title   string `json:"title,omitempty"`
	Date    string `json:"date,omitempty"`
	Summary string `json:"summary,omitempty"`
	// PrevRoute / NextRoute 记录上一篇/下一篇的路由.
	// 用于增量构建时检测文章关联 (上/下一篇) 是否变化: 即使内容未变化,
	// 只要关联变化 (新增/删除/改日期/改 slug) 就必须重新渲染.
	PrevRoute string `json:"prevRoute,omitempty"`
	NextRoute string `json:"nextRoute,omitempty"`
}

// BuildState 是 .plainship/state/build-state.json 的结构.
type BuildState struct {
	LastBuildID     string               `json:"lastBuildId"`
	RendererVersion string               `json:"rendererVersion"`
	ConfigHash      string               `json:"configHash"`
	ThemeHash       string               `json:"themeHash"`
	Files           map[string]FileState `json:"files"`
	// BuildNumber is the build number (ps-N) of this build, written by core.Build.
	BuildNumber string `json:"buildNumber,omitempty"`
	// CategoryHashes records content fingerprints of config/theme/docs at build time,
	// used by publish guard to verify build/ matches current sources.
	CategoryHashes map[string]string `json:"categoryHashes,omitempty"`
	// BasePath 是本次构建使用的链接基础路径 (dev 构建为空字符串, 生产构建取 site.url 的路径部分).
	// 用于检测 dev 与生产构建混用, 保证发布产物与部署方式一致.
	BasePath string `json:"basePath,omitempty"`
}

// NewBuildState 创建空的构建状态.
func NewBuildState() *BuildState {
	return &BuildState{Files: map[string]FileState{}, CategoryHashes: map[string]string{}}
}

// StateFilePath 返回构建状态文件路径.
func StateFilePath(spaceRoot string) string {
	return filepath.Join(StateDir(spaceRoot), "build-state.json")
}

// LoadState 读取构建状态. 文件不存在时返回空状态.
func LoadState(spaceRoot string) (*BuildState, error) {
	bs := NewBuildState()
	data, err := os.ReadFile(StateFilePath(spaceRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return bs, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, bs); err != nil {
		// 状态损坏时允许重新构建, 不阻塞用户.
		return NewBuildState(), nil
	}
	if bs.Files == nil {
		bs.Files = map[string]FileState{}
	}
	return bs, nil
}

// SaveState 写入构建状态.
func SaveState(spaceRoot string, bs *BuildState) error {
	if err := os.MkdirAll(StateDir(spaceRoot), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(bs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(StateFilePath(spaceRoot), data, 0o644)
}

// SyncState 是 .plainship/state/sync-state.json 的结构, 记录远程同步状态.
type SyncState struct {
	LastBuildID string            `json:"lastBuildId"`
	Files       map[string]string `json:"files"` // 相对 dist 路径 -> 哈希
	ServerURL   string            `json:"serverUrl"`
	SiteID      string            `json:"siteId"`
}

// SyncStateFilePath 返回同步状态文件路径.
func SyncStateFilePath(spaceRoot string) string {
	return filepath.Join(StateDir(spaceRoot), "sync-state.json")
}

// LoadSyncState 读取同步状态.
func LoadSyncState(spaceRoot string) *SyncState {
	ss := &SyncState{Files: map[string]string{}}
	data, err := os.ReadFile(SyncStateFilePath(spaceRoot))
	if err != nil {
		return ss
	}
	if err := json.Unmarshal(data, ss); err != nil {
		return &SyncState{Files: map[string]string{}}
	}
	if ss.Files == nil {
		ss.Files = map[string]string{}
	}
	return ss
}

// SaveSyncState 写入同步状态.
func SaveSyncState(spaceRoot string, ss *SyncState) error {
	if err := os.MkdirAll(StateDir(spaceRoot), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ss, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(SyncStateFilePath(spaceRoot), data, 0o644)
}
