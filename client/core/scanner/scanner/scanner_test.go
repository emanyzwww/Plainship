package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/papership-client/model/space"
)

// newSpace 创建一个临时 Space 根, 返回 Space 与根路径.
func newSpace(t *testing.T, mk func(root string)) *space.Space {
	t.Helper()
	root := t.TempDir()
	if mk != nil {
		mk(root)
	}
	return &space.Space{Root: root, Layout: space.DefaultLayout()}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasProblem(res *Result, path string) bool {
	for _, p := range res.Problems {
		if p.Path == path {
			return true
		}
	}
	return false
}

// findProblem 查找指定路径且严重级别匹配的问题; 严重级别用于锁定分级设计.
func findProblem(res *Result, path, severity string) (Problem, bool) {
	for _, p := range res.Problems {
		if p.Path == path && p.Severity == severity {
			return p, true
		}
	}
	return Problem{}, false
}

func docRelExists(res *Result, rel string) bool {
	for _, d := range res.Docs {
		if d.RelPath == rel {
			return true
		}
	}
	return false
}

func assetRelExists(res *Result, rel string) bool {
	for _, a := range res.Assets {
		if a.RelPath == rel {
			return true
		}
	}
	return false
}

func TestScanIndexesDocs(t *testing.T) {
	s := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
		writeFile(t, filepath.Join(root, "docs", "index.md"), "# Home\n")
		writeFile(t, filepath.Join(root, "docs", "guide", "intro.md"), "# Intro\n")
		writeFile(t, filepath.Join(root, "docs", "guide", "intro.zh.md"), "# 介绍\n")
		writeFile(t, filepath.Join(root, "docs", "about.markdown"), "# About\n")
	})

	res, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got := res.DocCount(); got != 4 {
		t.Fatalf("DocCount = %d, want 4", got)
	}
	if !res.ConfigPresent {
		t.Error("ConfigPresent = false, want true")
	}
	if res.ScannedAt <= 0 {
		t.Errorf("ScannedAt = %d, want > 0", res.ScannedAt)
	}

	// Stem 只剥离扩展名, 语言后缀保留给 downstream 解析.
	byPath := map[string]DocEntry{}
	for _, d := range res.Docs {
		byPath[d.RelPath] = d
	}
	intro := byPath["docs/guide/intro.zh.md"]
	if intro.Stem != "intro.zh" {
		t.Errorf("intro.zh.md Stem = %q, want intro.zh (语言后缀不做解析)", intro.Stem)
	}
	idx := byPath["docs/index.md"]
	if idx.Dir != "" {
		t.Errorf("docs/index.md Dir = %q, want empty", idx.Dir)
	}
	if intro.Dir != "guide" {
		t.Errorf("intro Dir = %q, want guide", intro.Dir)
	}

	// 排序稳定.
	for i := 1; i < len(res.Docs); i++ {
		if res.Docs[i-1].RelPath > res.Docs[i].RelPath {
			t.Errorf("docs not sorted: %s > %s", res.Docs[i-1].RelPath, res.Docs[i].RelPath)
		}
	}
}

func TestScanClassifiesAssets(t *testing.T) {
	s := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
		writeFile(t, filepath.Join(root, "docs", "index.md"), "# H\n")
		writeFile(t, filepath.Join(root, "docs", "img", "logo.png"), "png")
		writeFile(t, filepath.Join(root, "docs", "data.json"), "{}")
		writeFile(t, filepath.Join(root, "favicon.ico"), "ico")
		writeFile(t, filepath.Join(root, "robots.txt"), "disallow")
	})

	res, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if res.DocCount() != 1 {
		t.Errorf("DocCount = %d, want 1", res.DocCount())
	}
	if got := res.AssetCount(); got != 4 {
		t.Errorf("AssetCount = %d, want 4 (docs/img/logo.png, docs/data.json, favicon.ico, robots.txt)", got)
	}
	for i, a := range res.Assets {
		switch a.RelPath {
		case "docs/img/logo.png", "docs/data.json", "favicon.ico", "robots.txt":
		default:
			t.Errorf("unexpected asset %q", a.RelPath)
		}
		if i > 0 && res.Assets[i-1].RelPath > a.RelPath {
			t.Errorf("assets not sorted: %s > %s", res.Assets[i-1].RelPath, a.RelPath)
		}
	}
}

func TestScanSkipsInternalDirs(t *testing.T) {
	s := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
		writeFile(t, filepath.Join(root, "docs", "index.md"), "# H\n")
		writeFile(t, filepath.Join(root, "build", "index.html"), "<html>")
		writeFile(t, filepath.Join(root, ".papership", "config.yaml"), "cli_lang: zh\n")
		writeFile(t, filepath.Join(root, ".git", "HEAD"), "ref")
		writeFile(t, filepath.Join(root, ".DS_Store"), "junk")
		writeFile(t, filepath.Join(root, "docs", ".hidden.md"), "# hidden\n")
	})

	res, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if res.DocCount() != 1 {
		t.Errorf("DocCount = %d, want 1 (hidden dotfile excluded)", res.DocCount())
	}
	for _, a := range res.Assets {
		t.Errorf("asset %q should have been skipped (build/state/git/dotfile)", a.RelPath)
	}
	if !res.LocalConfigPresent {
		t.Error("LocalConfigPresent = false, want true (.papership/config.yaml exists)")
	}
}

func TestScanThemes(t *testing.T) {
	s := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
		writeFile(t, filepath.Join(root, "themes", "default", "theme.yaml"), "name: default\n")
		writeFile(t, filepath.Join(root, "themes", "night", "theme.yaml"), "name: night\n")
		writeFile(t, filepath.Join(root, "themes", "notes.txt"), "ignore this file\n")
	})

	res, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got := res.ThemeCount(); got != 2 {
		t.Fatalf("ThemeCount = %d, want 2", got)
	}
	if res.Themes[0].Name != "default" || res.Themes[1].Name != "night" {
		t.Errorf("themes not sorted: %+v", res.Themes)
	}

	// SkipThemes 选项.
	res2, err := ScanWithOptions(s, ScanOptions{SkipThemes: true})
	if err != nil {
		t.Fatalf("ScanWithOptions: %v", err)
	}
	if res2.ThemeCount() != 0 {
		t.Errorf("ThemeCount with SkipThemes = %d, want 0", res2.ThemeCount())
	}
}

func TestScanMissingDocsDirReportsProblem(t *testing.T) {
	s := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
	})

	res, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !hasProblem(res, s.DocsDir()) {
		t.Errorf("expected problem for missing docs dir %q, got %+v", s.DocsDir(), res.Problems)
	}
}

func TestScanMissingConfigReportsProblem(t *testing.T) {
	s := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "docs", "index.md"), "# H\n")
	})

	res, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if res.ConfigPresent {
		t.Error("ConfigPresent = true, want false")
	}
	if !hasProblem(res, s.ConfigPath()) {
		t.Errorf("expected problem for missing config %q", s.ConfigPath())
	}
	if res.LocalConfigPresent {
		t.Error("LocalConfigPresent = true, want false (no .papership/config.yaml)")
	}
}

// ancestorHasGit 从 start 沿父级向上查找 .git, 用于规避测试环境本身位于
// 某个 git 仓库内(例如 TMP 被指向仓库目录)导致的假阴性.
func ancestorHasGit(start string) bool {
	for dir := start; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
	}
}

func TestScanDetectsGitRoot(t *testing.T) {
	s := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
		if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(root, "docs", "index.md"), "# H\n")
	})

	res, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if res.Space.GitRoot != s.Root {
		t.Errorf("GitRoot = %q, want %q", res.Space.GitRoot, s.Root)
	}
	// 找到仓库后 GitAvailable 才被探测; 其值必须与本机 git 是否可用一致.
	_, gitErr := exec.LookPath("git")
	if want := gitErr == nil; res.Space.GitAvailable != want {
		t.Errorf("GitAvailable = %v, want %v (本机 git 探测结果)", res.Space.GitAvailable, want)
	}

	// 无 git 的场景: GitRoot 保持空; 且即使本机装有 git, GitAvailable 也必须是 false,
	// 因为可用性只在"找到仓库"之后才探测.
	s2 := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
	})
	if ancestorHasGit(s2.Root) {
		t.Skip("测试环境的临时目录位于某个 git 仓库内, 无法验证无 git 场景")
	}
	res2, err := Scan(s2)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if res2.Space.GitRoot != "" {
		t.Errorf("GitRoot = %q, want empty", res2.Space.GitRoot)
	}
	if res2.Space.GitAvailable {
		t.Error("GitAvailable = true without a git root, want false")
	}
}

func TestScanNormalizesLayout(t *testing.T) {
	s := &space.Space{Root: t.TempDir()} // Layout 为零值.
	writeFile(t, filepath.Join(s.Root, "docs", "index.md"), "# H\n")

	res, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if s.Layout.Docs != "docs" || s.Layout.State != ".papership" {
		t.Errorf("Layout not normalized: %+v", s.Layout)
	}
	if res.DocCount() != 1 {
		t.Errorf("DocCount = %d, want 1", res.DocCount())
	}
}

func TestScanMissingRootReturnsError(t *testing.T) {
	s := &space.Space{Root: filepath.Join(t.TempDir(), "does-not-exist"), Layout: space.DefaultLayout()}
	if _, err := Scan(s); err == nil {
		t.Fatal("Scan on missing root: expected error, got nil")
	}
}

func TestScanNilSpaceReturnsError(t *testing.T) {
	if _, err := Scan(nil); err == nil {
		t.Fatal("Scan(nil): expected error, got nil")
	}
}

// TestScanIncludeDotFiles 锁定 IncludeDotFiles 选项: 开启后点文件/点目录纳入索引,
// 但 .git 无论选项如何都无条件跳过.
func TestScanIncludeDotFiles(t *testing.T) {
	s := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
		writeFile(t, filepath.Join(root, "docs", "index.md"), "# H\n")
		writeFile(t, filepath.Join(root, "docs", ".hidden.md"), "# hidden\n")
		writeFile(t, filepath.Join(root, "docs", ".meta", "x.md"), "# meta\n")
		writeFile(t, filepath.Join(root, ".well-known", "acme.txt"), "t\n")
		writeFile(t, filepath.Join(root, ".DS_Store"), "junk\n")
		writeFile(t, filepath.Join(root, "themes", ".hidden-theme", "theme.yaml"), "name: hidden\n")
		writeFile(t, filepath.Join(root, ".git", "HEAD"), "ref\n")
	})

	// 默认: 点条目都被跳过, 只有非点文件 index.md 成为文档.
	res0, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if res0.DocCount() != 1 || !docRelExists(res0, "docs/index.md") {
		t.Errorf("default DocCount = %d, want 1 (仅非点文件 index.md)", res0.DocCount())
	}
	if res0.AssetCount() != 0 {
		t.Errorf("default AssetCount = %d, want 0 (点条目均跳过)", res0.AssetCount())
	}
	if res0.ThemeCount() != 0 {
		t.Errorf("default ThemeCount = %d, want 0", res0.ThemeCount())
	}

	// IncludeDotFiles=true: 点文件/点目录纳入索引.
	res1, err := ScanWithOptions(s, ScanOptions{IncludeDotFiles: true})
	if err != nil {
		t.Fatalf("ScanWithOptions: %v", err)
	}
	if got := res1.DocCount(); got != 3 {
		t.Errorf("includeDot DocCount = %d, want 3", got)
	}
	for _, rel := range []string{"docs/.hidden.md", "docs/.meta/x.md", "docs/index.md"} {
		if !docRelExists(res1, rel) {
			t.Errorf("includeDot missing doc %q", rel)
		}
	}
	if got := res1.AssetCount(); got != 2 {
		t.Errorf("includeDot AssetCount = %d, want 2", got)
	}
	for _, a := range res1.Assets {
		switch a.RelPath {
		case ".DS_Store", ".well-known/acme.txt":
		default:
			t.Errorf("unexpected asset %q", a.RelPath)
		}
		if strings.HasPrefix(a.RelPath, ".git") {
			t.Errorf(".git entry leaked into assets: %q", a.RelPath)
		}
	}
	if got := res1.ThemeCount(); got != 1 {
		t.Errorf("includeDot ThemeCount = %d, want 1", got)
	}
	if res1.Themes[0].Name != ".hidden-theme" {
		t.Errorf("theme = %q, want .hidden-theme", res1.Themes[0].Name)
	}
}

// TestScanEntryMeta 校验 DocEntry/AssetEntry 的元数据字段: 大小、修改时间、
// 扩展名归一、多级 Dir、绝对/相对路径. 这些字段是增量扫描的依据.
func TestScanEntryMeta(t *testing.T) {
	s := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
		writeFile(t, filepath.Join(root, "docs", "a", "b", "c.EN.MD"), "hello")
		writeFile(t, filepath.Join(root, "docs", "img", "logo.png"), "png")
	})

	res, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if res.DocCount() != 1 {
		t.Fatalf("DocCount = %d, want 1", res.DocCount())
	}
	d := res.Docs[0]
	wantAbs, _ := filepath.Abs(filepath.Join(s.Root, "docs", "a", "b", "c.EN.MD"))
	if d.AbsPath != wantAbs {
		t.Errorf("AbsPath = %q, want %q", d.AbsPath, wantAbs)
	}
	if d.RelPath != "docs/a/b/c.EN.MD" {
		t.Errorf("RelPath = %q, want docs/a/b/c.EN.MD", d.RelPath)
	}
	if d.Dir != "a/b" {
		t.Errorf("Dir = %q, want a/b (多级嵌套目录以 / 连接)", d.Dir)
	}
	if d.Base != "c.EN.MD" {
		t.Errorf("Base = %q, want c.EN.MD", d.Base)
	}
	if d.Stem != "c.EN" {
		t.Errorf("Stem = %q, want c.EN (只剥离扩展名)", d.Stem)
	}
	if d.Ext != ".md" {
		t.Errorf("Ext = %q, want .md (扩展名归一为小写)", d.Ext)
	}
	info, err := os.Stat(wantAbs)
	if err != nil {
		t.Fatal(err)
	}
	if d.Size != info.Size() {
		t.Errorf("Size = %d, want %d", d.Size, info.Size())
	}
	if d.ModTime != info.ModTime().Unix() {
		t.Errorf("ModTime = %d, want %d", d.ModTime, info.ModTime().Unix())
	}

	// Asset 元数据同步校验.
	if res.AssetCount() != 1 {
		t.Fatalf("AssetCount = %d, want 1", res.AssetCount())
	}
	a := res.Assets[0]
	wantAssetAbs, _ := filepath.Abs(filepath.Join(s.Root, "docs", "img", "logo.png"))
	if a.AbsPath != wantAssetAbs {
		t.Errorf("asset AbsPath = %q, want %q", a.AbsPath, wantAssetAbs)
	}
	if a.RelPath != "docs/img/logo.png" {
		t.Errorf("asset RelPath = %q, want docs/img/logo.png", a.RelPath)
	}
	ainfo, err := os.Stat(wantAssetAbs)
	if err != nil {
		t.Fatal(err)
	}
	if a.Size != ainfo.Size() {
		t.Errorf("asset Size = %d, want %d", a.Size, ainfo.Size())
	}
	if a.ModTime != ainfo.ModTime().Unix() {
		t.Errorf("asset ModTime = %d, want %d", a.ModTime, ainfo.ModTime().Unix())
	}
}

// TestScanProblemSeverities 锁定 Problem 的严重级别分级, 以及几个边界形态:
// config 路径为目录(存在但形态不对)、themes 目录存在但无任何主题目录(不报 warning).
func TestScanProblemSeverities(t *testing.T) {
	// 缺 docs 目录 → error; 缺 themes 目录 → warning.
	s := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
	})
	res, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if _, ok := findProblem(res, s.DocsDir(), SeverityError); !ok {
		t.Errorf("missing docs: want error problem at %q, got %+v", s.DocsDir(), res.Problems)
	}
	if _, ok := findProblem(res, s.ThemesDir(), SeverityWarning); !ok {
		t.Errorf("missing themes: want warning problem at %q, got %+v", s.ThemesDir(), res.Problems)
	}

	// 缺配置文件 → error.
	s2 := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "docs", "index.md"), "# H\n")
	})
	res2, err := Scan(s2)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if _, ok := findProblem(res2, s2.ConfigPath(), SeverityError); !ok {
		t.Errorf("missing config: want error problem at %q, got %+v", s2.ConfigPath(), res2.Problems)
	}

	// themes 目录存在但只有散落文件 → 不产生 warning(与"目录缺失报 warning"不对称, 属设计约定).
	s3 := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
		writeFile(t, filepath.Join(root, "docs", "index.md"), "# H\n")
		writeFile(t, filepath.Join(root, "themes", "notes.txt"), "x\n")
	})
	res3, err := Scan(s3)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if _, ok := findProblem(res3, s3.ThemesDir(), SeverityWarning); ok {
		t.Errorf("themes dir present but empty of themes: unexpected warning, got %+v", res3.Problems)
	}

	// 配置文件路径是目录 → ConfigPresent=false + error 问题.
	s4 := newSpace(t, func(root string) {
		if err := os.MkdirAll(filepath.Join(root, "papership.yaml"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(root, "docs", "index.md"), "# H\n")
	})
	res4, err := Scan(s4)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if res4.ConfigPresent {
		t.Error("ConfigPresent = true for a directory, want false")
	}
	if _, ok := findProblem(res4, s4.ConfigPath(), SeverityError); !ok {
		t.Errorf("config-as-dir: want error problem at %q, got %+v", s4.ConfigPath(), res4.Problems)
	}
}

// TestScanRootMdIsAsset 锁定设计: 文档只在 docs 目录下, 根目录散落的 .md 属于静态资源.
func TestScanRootMdIsAsset(t *testing.T) {
	s := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
		writeFile(t, filepath.Join(root, "README.md"), "# root readme\n")
		writeFile(t, filepath.Join(root, "docs", "index.md"), "# H\n")
	})

	res, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if res.DocCount() != 1 {
		t.Errorf("DocCount = %d, want 1 (仅 docs 下的 md 为文档)", res.DocCount())
	}
	if docRelExists(res, "README.md") {
		t.Error("README.md indexed as doc, want asset")
	}
	if !assetRelExists(res, "README.md") {
		t.Error("README.md missing from assets")
	}
}

// TestScanIdempotent 校验 README 承诺的幂等性: 同一 Space 可被重复安全扫描,
// 文档/资源/主题不会因重复调用而重复累积.
func TestScanIdempotent(t *testing.T) {
	s := newSpace(t, func(root string) {
		writeFile(t, filepath.Join(root, "papership.yaml"), "site_id: demo\n")
		writeFile(t, filepath.Join(root, "docs", "index.md"), "# H\n")
		writeFile(t, filepath.Join(root, "docs", "guide", "intro.md"), "# I\n")
		writeFile(t, filepath.Join(root, "themes", "default", "theme.yaml"), "name: default\n")
	})

	r1, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan #1: %v", err)
	}
	r2, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan #2: %v", err)
	}
	if r1.DocCount() != 2 || r2.DocCount() != 2 {
		t.Errorf("idempotence broken: r1.DocCount=%d r2.DocCount=%d, want both 2", r1.DocCount(), r2.DocCount())
	}
	if r1.AssetCount() != 0 || r2.AssetCount() != 0 {
		t.Errorf("idempotence broken: r1.AssetCount=%d r2.AssetCount=%d, want both 0", r1.AssetCount(), r2.AssetCount())
	}
	if r1.ThemeCount() != 1 || r2.ThemeCount() != 1 {
		t.Errorf("idempotence broken: r1.ThemeCount=%d r2.ThemeCount=%d, want both 1", r1.ThemeCount(), r2.ThemeCount())
	}
}

// TestScanPartialLayoutNormalization 校验 normalizeLayout 按字段回填:
// 已设置的字段保持原值, 未设置的字段回填默认值.
func TestScanPartialLayoutNormalization(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "site.yaml"), "site_id: demo\n")
	writeFile(t, filepath.Join(root, "content", "index.md"), "# H\n")
	s := &space.Space{
		Root: root,
		Layout: space.Layout{
			Docs:   "content",
			Themes: "skins",
			Config: "site.yaml",
		},
	}
	res, err := Scan(s)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if s.Layout.Docs != "content" || s.Layout.Themes != "skins" || s.Layout.Config != "site.yaml" {
		t.Errorf("explicit layout fields overwritten: %+v", s.Layout)
	}
	if s.Layout.Build != "build" || s.Layout.State != ".papership" {
		t.Errorf("zero layout fields not backfilled: %+v", s.Layout)
	}
	if res.DocCount() != 1 {
		t.Errorf("DocCount = %d, want 1 (content/index.md)", res.DocCount())
	}
	// custom themes dir 缺失时按 Layout 报告; 且不得出现 error 级问题.
	if _, ok := findProblem(res, filepath.Join(root, "skins"), SeverityWarning); !ok {
		t.Errorf("missing slogan for absent skins dir, got %+v", res.Problems)
	}
	for _, p := range res.Problems {
		if p.Severity == SeverityError {
			t.Errorf("unexpected error problem: %+v", p)
		}
	}
}

// TestKindString 锁定 Kind 枚举到字符串的映射; 未定义值与 KindUnknown 一致.
func TestKindString(t *testing.T) {
	cases := []struct {
		kind Kind
		want string
	}{
		{KindDoc, "doc"},
		{KindTheme, "theme"},
		{KindAsset, "asset"},
		{KindUnknown, "unknown"},
		{Kind(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.kind.String(); got != c.want {
			t.Errorf("Kind(%d).String() = %q, want %q", c.kind, got, c.want)
		}
	}
}
