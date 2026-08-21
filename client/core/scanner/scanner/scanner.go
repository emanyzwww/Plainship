// Package scanner 负责扫描单个 PaperShip Space 下的文档, 主题, 配置等, 用于构建
// PaperShip Site 的相关资源文件.
package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/emanyzwww/papership-client/core/pipeline"
	"github.com/emanyzwww/papership-client/model/space"
)

// 可注入的测试替身.
var (
	osStat  = os.Stat
	walkDir = filepath.WalkDir
)

// stageName 是本阶段的问题来源标记.
const stageName = "scanner"

// ScanOptions 控制扫描行为; 零值即默认行为.
type ScanOptions struct {
	SkipThemes      bool // SkipThemes 为 true 时跳过 themes 主题清单收集, 默认 false.
	IncludeDotFiles bool // IncludeDotFiles 为 true 时包含 . 开头的文件和目录, 默认 false.
}

// Stage 是扫描阶段: 实现 pipeline.Stage, 供编排层串联; 零值可用 (默认选项).
type Stage struct{}

// Run 执行一次带上下文的扫描.
func (Stage) Run(ctx context.Context, in *space.Space) (*Result, error) { return Scan(ctx, in) }

// Scan 执行一次完整扫描, 上下文取消时中止.
func Scan(ctx context.Context, s *space.Space) (*Result, error) {
	return ScanWithOptions(ctx, s, ScanOptions{})
}

// ScanWithOptions 与 Scan 相同, 支持自定义扫描选项.
func ScanWithOptions(ctx context.Context, s *space.Space, opts ScanOptions) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("scanner: nil space")
	}

	root, err := filepath.Abs(s.Root)
	if err != nil {
		return nil, fmt.Errorf("scanner: resolve root %q: %w", s.Root, err)
	}

	if _, err := osStat(root); err != nil {
		return nil, fmt.Errorf("scanner: space root %q: %w", root, err)
	}

	s.Root = root
	s.Layout = normalizeLayout(s.Layout)

	res := &Result{
		Space:     s,
		ScannedAt: time.Now().Unix(),
	}

	detectGit(res)
	detectConfigFiles(res)

	scanDocs(res, opts)
	scanThemes(res, opts)
	scanRootAssets(res, opts)

	pipeline.SortByKey(res.Docs)
	pipeline.SortByKey(res.Assets)
	sortThemes(res.Themes)

	return res, nil
}

// normalizeLayout 用标准布局回填 Layout 中为空的字段.
func normalizeLayout(l space.Layout) space.Layout {
	d := space.DefaultLayout()
	if l.Docs == "" {
		l.Docs = d.Docs
	}
	if l.Themes == "" {
		l.Themes = d.Themes
	}
	if l.Build == "" {
		l.Build = d.Build
	}
	if l.State == "" {
		l.State = d.State
	}
	if l.Config == "" {
		l.Config = d.Config
	}
	return l
}

// ==============================
// Git 与配置探测.
// ==============================

// detectGit 探测 Git 仓库根目录与 git 可执行文件是否可用, 回填到 Space.
func detectGit(res *Result) {
	s := res.Space
	if gitRoot, err := findGitRoot(s.Root); err == nil && gitRoot != "" {
		s.GitRoot = gitRoot
		_, err := exec.LookPath("git")
		s.GitAvailable = err == nil
	}
}

// findGitRoot 从 start 开始逐级向上查找含 .git 的目录; 未找到返回空串.
func findGitRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := osStat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// detectConfigFiles 检测配置文件是否存在并记录到 Result.
//
// 注意: 这里只做存在性检测, 不解析 YAML. 配置解析属于配置加载层 (后续可接入 yaml 库),
// scanner 仅负责把"缺配置"作为问题暴露出来.
func detectConfigFiles(res *Result) {
	s := res.Space

	if info, err := osStat(s.ConfigPath()); err == nil && !info.IsDir() {
		res.ConfigPresent = true
	} else if info != nil && info.IsDir() {
		res.Problems = append(res.Problems, pipeline.Problemf(SeverityError, stageName, s.ConfigPath(), "Space 配置文件路径是一个目录而非文件, 将以默认配置继续; 若要发布站点请先创建 papership.yaml"))
	} else {
		res.Problems = append(res.Problems, pipeline.Problemf(SeverityError, stageName, s.ConfigPath(), "Space 配置文件不存在, 将以默认配置继续; 若要发布站点请先补全 papership.yaml"))
	}

	localPath := filepath.Join(s.StateDir(), "config.yaml")
	if info, err := osStat(localPath); err == nil && !info.IsDir() {
		res.LocalConfigPresent = true
	}
}

// ==============================
// docs 扫描.
// ==============================

// scanDocs 遍历 docs 目录: .md 归为文档, 其余文件归为静态资源.
func scanDocs(res *Result, opts ScanOptions) {
	s := res.Space
	docsDir := s.DocsDir()

	// 处理 docs 不存在.
	if info, err := osStat(docsDir); err != nil || !info.IsDir() {
		res.Problems = append(res.Problems, pipeline.Problemf(SeverityError, stageName, docsDir, "docs 目录不存在或不可读, 站点将无任何文档"))
		return
	}

	err := walkDir(docsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			res.Problems = append(res.Problems, pipeline.Problemf(SeverityWarning, stageName, path, "遍历失败: %v", err))
			return nil // 继续遍历兄弟节点.
		}
		if d.IsDir() {
			if shouldSkipName(d.Name(), opts.IncludeDotFiles) {
				return fs.SkipDir
			}
			return nil
		}
		if shouldSkipName(d.Name(), opts.IncludeDotFiles) {
			return nil
		}

		relDocs, rerr := filepath.Rel(docsDir, path)
		if rerr != nil {
			return nil
		}
		if isDocFile(d.Name()) {
			if entry, ok := classifyDoc(s, path, relDocs, d); ok {
				res.Docs = append(res.Docs, entry)
			}
			return nil
		}
		if entry, ok := classifyAsset(s, path, d); ok {
			res.Assets = append(res.Assets, entry)
		}
		return nil
	})
	if err != nil {
		res.Problems = append(res.Problems, pipeline.Problemf(SeverityError, stageName, docsDir, "扫描 docs 失败: %v", err))
	}
}

// shouldSkipName 判断是否跳过该名字: ".git" 无论选项如何都跳过.
//
// 其余点开头的条目仅在 IncludeDotFiles=false (默认值) 时跳过.
func shouldSkipName(name string, includeDotFiles bool) bool {
	if name == ".git" {
		return true
	}
	if !includeDotFiles && strings.HasPrefix(name, ".") {
		return true
	}
	return false
}

// isDocFile 判断是否为文档文件: .md / .markdown.
func isDocFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".md" || ext == ".markdown"
}

// classifyDoc 根据路径与目录项构造 DocEntry.
//
// relDocs 是相对 docs 根目录的路径, 返回的 Dir 统一转为 "/" 分隔;
// RelPath 相对 Space 根目录.
//
// Stem 只剥离扩展名.
func classifyDoc(s *space.Space, path, relDocs string, d fs.DirEntry) (DocEntry, bool) {
	name := d.Name()
	ext := strings.ToLower(filepath.Ext(name))
	stem := name[:len(name)-len(ext)]

	info, err := d.Info()
	if err != nil {
		return DocEntry{}, false
	}

	relParts := strings.Split(filepath.ToSlash(relDocs), "/")
	dir := ""
	if len(relParts) > 1 {
		dir = strings.Join(relParts[:len(relParts)-1], "/")
	}

	relRoot, rerr := filepath.Rel(s.Root, path)
	if rerr != nil {
		return DocEntry{}, false
	}
	abs, _ := filepath.Abs(path)
	return DocEntry{
		AbsPath: abs,
		RelPath: filepath.ToSlash(relRoot),
		Dir:     dir,
		Base:    name,
		Stem:    stem,
		Ext:     ext,
		Size:    info.Size(),
		ModTime: info.ModTime().Unix(),
	}, true
}

// classifyAsset 构造 AssetEntry.
func classifyAsset(s *space.Space, path string, d fs.DirEntry) (AssetEntry, bool) {
	info, err := d.Info()
	if err != nil {
		return AssetEntry{}, false
	}
	rel, rerr := filepath.Rel(s.Root, path)
	if rerr != nil {
		return AssetEntry{}, false
	}
	abs, _ := filepath.Abs(path)
	return AssetEntry{
		AbsPath: abs,
		RelPath: filepath.ToSlash(rel),
		Size:    info.Size(),
		ModTime: info.ModTime().Unix(),
	}, true
}

// ==============================
// themes 扫描.
// ==============================

// scanThemes 遍历 themes 目录的一级条目, 每个一级目录视为一个主题.
func scanThemes(res *Result, opts ScanOptions) {
	if opts.SkipThemes {
		return
	}
	s := res.Space
	themesDir := s.ThemesDir()

	if info, err := osStat(themesDir); err != nil || !info.IsDir() {
		res.Problems = append(res.Problems, pipeline.Problemf(SeverityWarning, stageName, themesDir, "themes 目录不存在, 将使用默认主题"))
		return
	}

	entries, err := os.ReadDir(themesDir)
	if err != nil {
		res.Problems = append(res.Problems, pipeline.Problemf(SeverityWarning, stageName, themesDir, "读取 themes 失败: %v", err))
		return
	}

	for _, e := range entries {
		if shouldSkipName(e.Name(), opts.IncludeDotFiles) {
			continue
		}
		// 主题以一级目录为准.
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(themesDir, e.Name())
		rel, rerr := filepath.Rel(s.Root, path)
		if rerr != nil {
			continue
		}
		res.Themes = append(res.Themes, ThemeEntry{
			Name:    e.Name(),
			AbsPath: path,
			RelPath: filepath.ToSlash(rel),
		})
	}
}

// ==============================
// 根目录静态资源扫描.
// ==============================

// scanRootAssets 遍历 Space 根目录, 收集散落的静态资源.
func scanRootAssets(res *Result, opts ScanOptions) {
	s := res.Space

	// 需要跳过的子树.
	var skipRoots []string
	for _, dir := range []string{s.DocsDir(), s.ThemesDir(), s.BuildDir(), s.StateDir()} {
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		skipRoots = append(skipRoots, abs)
	}

	// 根目录散落的文件不收集: 配置文件本身不作为静态资源.
	configAbs, err := filepath.Abs(s.ConfigPath())
	if err == nil {
		skipRoots = append(skipRoots, configAbs)
	}

	err = walkDir(s.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			res.Problems = append(res.Problems, pipeline.Problemf(SeverityWarning, stageName, path, "遍历失败: %v", err))
			return nil
		}
		if underPath(path, skipRoots...) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if shouldSkipName(d.Name(), opts.IncludeDotFiles) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if entry, ok := classifyAsset(s, path, d); ok {
			res.Assets = append(res.Assets, entry)
		}
		return nil
	})
	if err != nil {
		res.Problems = append(res.Problems, pipeline.Problemf(SeverityWarning, stageName, s.Root, "扫描根目录失败: %v", err))
	}
}

// underPath 判断 path 是否等于任一 base 或位于其子树内.
func underPath(path string, bases ...string) bool {
	for _, b := range bases {
		if path == b {
			return true
		}
		if strings.HasPrefix(path, b+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// ==============================
// 排序.
// ==============================

// sortThemes 按主题名排序; 主题无 RelPath 键, 故保留本地实现.
func sortThemes(themes []ThemeEntry) {
	sort.Slice(themes, func(i, j int) bool { return themes[i].Name < themes[j].Name })
}
