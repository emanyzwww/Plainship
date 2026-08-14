// Package revision 封装 Plainship 的版本控制语义:
//   - 源码类别划分 (config / theme / docs) 与变更统计
//   - 类别内容指纹 (提交信息与 publish 守卫的依据)
//   - 机器提交信息协议 (<类别> build=<编号> hash=<指纹>)
//   - 构建编号管理 (ps-N tag)
//
// 通用 Git 命令调用由 internal/git 提供, 本包只承载 Plainship 语义.
package revision

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/emanyzwww/Plainship/internal/fsutil"
	"github.com/emanyzwww/Plainship/internal/git"
	"github.com/emanyzwww/Plainship/internal/hash"
	"github.com/emanyzwww/Plainship/internal/i18n"
	"github.com/emanyzwww/Plainship/internal/layout"
	"github.com/emanyzwww/Plainship/internal/space"
)

// Category 是源码类别, 分步提交与变更统计均按类别进行.
type Category string

const (
	// CategoryConfig 是根目录配置 (plainship.yaml + .gitignore).
	CategoryConfig Category = "config"
	// CategoryTheme 是 themes 目录.
	CategoryTheme Category = "theme"
	// CategoryDocs 是 docs 目录.
	CategoryDocs Category = "docs"
)

// Categories 固定类别的处理顺序: config -> theme -> docs.
var Categories = []Category{CategoryConfig, CategoryTheme, CategoryDocs}

// CategoryChanges 是某一类别的 Git 变更统计.
type CategoryChanges struct {
	Added    int
	Modified int
	Deleted  int
	Paths    []string
}

// HasChanges 判断该类别是否有任何变更.
func (c CategoryChanges) HasChanges() bool {
	return c.Added > 0 || c.Modified > 0 || c.Deleted > 0
}

// GitSummary 是 Space 相关的 Git 状态摘要.
type GitSummary struct {
	Available bool
	IsRepo    bool
	Branch    string
	Clean     bool
	Changes   map[Category]CategoryChanges
}

// GitStatus 收集 Space 相关的 Git 状态, 按 config/theme/docs 分类统计.
func GitStatus(s *space.Space) GitSummary {
	gs := GitSummary{Available: git.Available(), Changes: map[Category]CategoryChanges{}}
	for _, cat := range Categories {
		gs.Changes[cat] = CategoryChanges{}
	}
	if !gs.Available || s.GitRoot == "" {
		return gs
	}
	gs.IsRepo = true
	gs.Branch = git.Branch(s.Root)
	entries, err := git.Porcelain(s.Root)
	if err != nil {
		return gs
	}
	// Space 相对 Git 根的路径前缀 (monorepo 场景).
	// 注意: Windows 下 git 返回长路径而 filepath.Abs 可能是 8.3 短路径,
	// 不能直接用字符串比较判断是否同一目录; 用 Rel 结果判定祖先关系.
	spacePrefix := ""
	if rel, err := filepath.Rel(s.GitRoot, s.Root); err == nil &&
		rel != "." && !strings.HasPrefix(rel, "..") {
		spacePrefix = filepath.ToSlash(rel)
	}
	classify := func(p string) Category {
		if spacePrefix != "" {
			if !strings.HasPrefix(p, spacePrefix+"/") {
				return ""
			}
			p = strings.TrimPrefix(p, spacePrefix+"/")
		}
		switch {
		case p == layout.ConfigFile || p == layout.GitignoreFile:
			return CategoryConfig
		case strings.HasPrefix(p, layout.ThemesDir+"/"):
			return CategoryTheme
		case strings.HasPrefix(p, layout.DocsDir+"/"):
			return CategoryDocs
		}
		return ""
	}
	for _, e := range entries {
		cat := classify(e.Path)
		if cat == "" {
			continue
		}
		c := gs.Changes[cat]
		code := strings.TrimSpace(e.Status)
		switch {
		case code == "??" || strings.HasPrefix(code, "A"):
			c.Added++
		case strings.HasPrefix(code, "M") || strings.HasPrefix(code, "R") || strings.HasPrefix(code, "C"):
			c.Modified++
		case strings.HasPrefix(code, "D"):
			c.Deleted++
		default:
			continue
		}
		c.Paths = append(c.Paths, e.Path)
		gs.Changes[cat] = c
	}
	gs.Clean = len(entries) == 0
	return gs
}

// CategoryHash 计算某一类别的联合内容指纹.
// 同一类别内容相同则指纹相同, 用于提交信息与 publish 守卫.
func CategoryHash(s *space.Space, cat Category) (string, error) {
	switch cat {
	case CategoryConfig:
		// config 类别包含根目录配置文件: plainship.yaml + .gitignore.
		inputs := map[string]string{}
		for _, name := range []string{layout.ConfigFile, layout.GitignoreFile} {
			p := filepath.Join(s.Root, name)
			if !fsutil.Exists(p) {
				continue
			}
			h, err := hash.File(p)
			if err != nil {
				return "", err
			}
			inputs[name] = h
		}
		return hash.Inputs(inputs), nil
	case CategoryTheme:
		return hashDir(s.ThemesDir())
	case CategoryDocs:
		return hashDir(s.DocsDir())
	}
	return "", i18n.Errorf(i18n.RevisionUnknownCat, cat)
}

// hashDir 计算目录下全部文件的联合指纹 (路径排序后逐文件哈希).
func hashDir(dir string) (string, error) {
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

// commitSubjectPattern 解析机器提交信息: <type> build=<编号> hash=<指纹>.
var commitSubjectPattern = regexp.MustCompile(`^([a-z]+) build=(\S+) hash=([0-9a-f]+)`)

// CommitMessage 生成某一类别的机器提交信息.
func CommitMessage(cat Category, buildNumber, catHash string) string {
	return fmt.Sprintf("%s build=%s hash=%s", cat, buildNumber, shortHash(catHash))
}

// shortHash 截短哈希到 16 位, 便于阅读且足够校验.
func shortHash(h string) string {
	if len(h) > 16 {
		return h[:16]
	}
	return h
}

// LatestCategoryCommit 查询某类别最新提交中的 build 编号与 hash.
func LatestCategoryCommit(s *space.Space, cat Category) (buildNumber, catHash string, ok bool) {
	subject, found := git.LatestCommitSubject(s.Root, "^"+string(cat)+" build=")
	if !found {
		return "", "", false
	}
	m := commitSubjectPattern.FindStringSubmatch(subject)
	if m == nil {
		return "", "", false
	}
	return m[2], m[3], true
}

// NextBuildNumber 计算下一个构建编号, 格式 ps-0001.
func NextBuildNumber(dir string) (string, error) {
	return git.NextBuildNumber(dir)
}

// TagBuild 在 HEAD 打当前构建编号 tag.
func TagBuild(dir, buildNumber string) error {
	return git.Tag(dir, buildNumber)
}

// CommitPaths 创建一次只包含指定路径的提交, 用于分步提交.
func CommitPaths(dir, message string, paths ...string) error {
	return git.CommitPaths(dir, message, paths...)
}

// IsRepo 判断 Space 是否位于 Git 仓库中.
func IsRepo(s *space.Space) bool {
	return git.IsRepo(s.Root)
}

// TagsAtHEAD 返回 HEAD 上的 Plainship 构建编号 tag 列表.
func TagsAtHEAD(dir string) ([]string, error) {
	return git.TagsAtHEAD(dir, git.BuildTagPrefix)
}
