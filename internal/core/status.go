// Package core 是 Plainship 的核心编排层.
// 只负责流程编排 (CreateSpace / Build / Publish / Status / Dev),
// Git 语义 (类别划分, 指纹, 提交协议, 编号) 由 internal/revision 提供.
package core

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/emanyzwww/plainship/internal/fsutil"
	"github.com/emanyzwww/plainship/internal/hash"
	"github.com/emanyzwww/plainship/internal/layout"
	"github.com/emanyzwww/plainship/internal/manifest"
	"github.com/emanyzwww/plainship/internal/revision"
	"github.com/emanyzwww/plainship/internal/space"
	"github.com/emanyzwww/plainship/internal/state"
	"github.com/emanyzwww/plainship/internal/version"
)

// StatusReport 是 status 命令的输出数据.
type StatusReport struct {
	SpaceRoot      string
	GitAvailable   bool
	GitBranch      string
	GitClean       bool
	HasRepo        bool
	Changes        map[revision.Category]revision.CategoryChanges
	LastBuildID    string
	LastBuildTime  string
	BuildOutdated  bool
	DocCount       int
	BuildNumber    string
	ServerURL      string
	PublishedBuild string
}

// Status 收集 Space 的完整状态.
func Status(spaceRoot string) (*StatusReport, error) {
	s, err := space.Load(spaceRoot)
	if err != nil {
		return nil, err
	}
	rep := &StatusReport{SpaceRoot: s.Root, ServerURL: s.Config.Server.URL}
	gs := revision.GitStatus(s)
	rep.GitAvailable = gs.Available
	rep.HasRepo = gs.IsRepo
	rep.GitBranch = gs.Branch
	rep.GitClean = gs.Clean
	rep.Changes = gs.Changes

	bs, err := state.LoadState(s.Root)
	if err == nil && bs.LastBuildID != "" {
		rep.LastBuildID = bs.LastBuildID
		rep.BuildNumber = bs.BuildNumber
		if m, err := manifest.Read(s.Root, bs.LastBuildID); err == nil {
			rep.LastBuildTime = m.BuiltAt
			rep.DocCount = countPages(m)
		}
	}
	if rep.BuildNumber == "" {
		if tags, err := revision.TagsAtHEAD(s.Root); err == nil && len(tags) > 0 {
			rep.BuildNumber = tags[len(tags)-1]
		}
	}
	ss := state.LoadSyncState(s.Root)
	rep.PublishedBuild = ss.LastBuildID
	rep.BuildOutdated = buildOutdated(s, bs)
	return rep, nil
}

// buildOutdated 通过比较配置/主题/渲染器/内容哈希判断构建是否过期.
// 覆盖: 配置哈希, 渲染器版本, Space 主题目录, docs 下全部文件 (md 与资源).
func buildOutdated(s *space.Space, bs *state.BuildState) bool {
	if bs == nil || bs.LastBuildID == "" {
		return true
	}
	// 渲染器版本变化 (升级 Plainship) 视为过期.
	if bs.RendererVersion != version.RendererVersion() {
		return true
	}
	cfgHash, err := s.Config.Hash()
	if err != nil || cfgHash != bs.ConfigHash {
		return true
	}
	// Space 主题变化; 内嵌主题由 RendererVersion 覆盖.
	th, err := currentThemeHash(s)
	if err != nil {
		return true
	}
	if th != "" && bs.ThemeHash != "" && th != bs.ThemeHash {
		return true
	}
	// 当前全部文件 (md + 资源) 必须与状态一一对应且哈希一致.
	current, err := scanDocsHashes(s.DocsDir())
	if err != nil {
		return true
	}
	if len(current) != len(bs.Files) {
		return true
	}
	for rel, h := range current {
		fs, ok := bs.Files[rel]
		if !ok || fs.Hash != h {
			return true
		}
	}
	return false
}

// currentThemeHash 计算当前 Space 主题目录的联合哈希.
// 主题目录不存在 (使用内嵌主题) 时返回空字符串, 由 RendererVersion 覆盖.
func currentThemeHash(s *space.Space) (string, error) {
	dir := filepath.Join(s.ThemesDir(), s.Config.Theme.Name)
	if !fsutil.IsDir(dir) {
		return "", nil
	}
	inputs := map[string]string{}
	files, err := fsutil.ListFiles(dir)
	if err != nil {
		return "", err
	}
	for _, rel := range files {
		h, err := hash.File(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		inputs[rel] = h
	}
	return hash.Inputs(inputs), nil
}

// scanDocsHashes 扫描 docs 目录全部文件 (与 builder 相同的隐藏规则) 并计算哈希.
func scanDocsHashes(docsDir string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != docsDir && strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(docsDir, path)
		if err != nil {
			return err
		}
		h, err := hash.File(path)
		if err != nil {
			return err
		}
		out[layout.DocsDir+"/"+filepath.ToSlash(rel)] = h
		return nil
	})
	return out, err
}

func countPages(m *manifest.Manifest) int {
	n := 0
	for _, f := range m.Files {
		if f.Type == manifest.TypePage {
			n++
		}
	}
	return n
}
