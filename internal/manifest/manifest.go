// Package manifest 定义构建清单 (Build Manifest).
// 清单用途: 增量构建, 删除检测, 同步, 调试, rollback.
package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/emanyzwww/Plainship/internal/state"
)

// FileType 表示产物类型.
type FileType string

const (
	// TypePage 是文档页面.
	TypePage FileType = "page"
	// TypeIndex 是首页或列表页.
	TypeIndex FileType = "index"
	// TypeAsset 是静态资源文件.
	TypeAsset FileType = "asset"
	// TypeSEO 是 SEO 文件.
	TypeSEO FileType = "seo"
)

// FileEntry 描述一个构建产物.
type FileEntry struct {
	Source string   `json:"source"` // 源文件路径(相对 Space), 页面才有值
	Output string   `json:"output"` // 输出路径(相对 dist)
	Hash   string   `json:"hash"`   // 内容哈希
	Type   FileType `json:"type"`
}

// Manifest 是一次构建的完整清单.
type Manifest struct {
	BuildID    string      `json:"buildId"`
	SiteID     string      `json:"siteId"`
	BuiltAt    string      `json:"builtAt"`
	Files      []FileEntry `json:"files"`
	Deleted    []FileEntry `json:"deleted"`
	SourceHash string      `json:"sourceHash"`
}

// New 创建清单实例.
func New(buildID, siteID, sourceHash string) *Manifest {
	return &Manifest{
		BuildID:    buildID,
		SiteID:     siteID,
		BuiltAt:    time.Now().Format(time.RFC3339),
		Files:      []FileEntry{},
		Deleted:    []FileEntry{},
		SourceHash: sourceHash,
	}
}

// Add 追加一个产物条目.
func (m *Manifest) Add(entry FileEntry) {
	m.Files = append(m.Files, entry)
}

// AddDeleted 追加一个被删除的产物条目.
func (m *Manifest) AddDeleted(entry FileEntry) {
	m.Deleted = append(m.Deleted, entry)
}

// FileMap 返回 output -> FileEntry 的映射, 便于查找.
func (m *Manifest) FileMap() map[string]FileEntry {
	out := make(map[string]FileEntry, len(m.Files))
	for _, f := range m.Files {
		out[f.Output] = f
	}
	return out
}

// Path 返回清单文件路径.
func Path(spaceRoot, buildID string) string {
	return filepath.Join(state.ManifestsDir(spaceRoot), buildID+".json")
}

// Write 将清单写入 .plainship/manifests.
func Write(spaceRoot string, m *Manifest) error {
	if err := os.MkdirAll(state.ManifestsDir(spaceRoot), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(spaceRoot, m.BuildID), data, 0o644)
}

// Read 读取指定构建的清单.
func Read(spaceRoot, buildID string) (*Manifest, error) {
	data, err := os.ReadFile(Path(spaceRoot, buildID))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Latest 读取最近的清单.
// 依据 .plainship/state/build-state.json 中的 lastBuildId.
func Latest(spaceRoot string) (*Manifest, error) {
	bs, err := state.LoadState(spaceRoot)
	if err != nil {
		return nil, err
	}
	if bs.LastBuildID == "" {
		return nil, os.ErrNotExist
	}
	return Read(spaceRoot, bs.LastBuildID)
}

// Load 加载最近一次成功构建的清单, 缺失时返回空清单.
func Load(spaceRoot string) (*Manifest, error) {
	m, err := Latest(spaceRoot)
	if err != nil {
		return New("", "", ""), nil
	}
	return m, nil
}
