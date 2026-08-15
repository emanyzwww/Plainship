package builder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emanyzwww/plainship/internal/space"
)

// setupSpace 在临时目录创建一个最小 Space.
func setupSpace(t *testing.T) *space.Space {
	t.Helper()
	root := t.TempDir()
	s, err := space.Create(root)
	if err != nil {
		t.Fatalf("创建 Space 失败: %v", err)
	}
	return s
}

func writeDoc(t *testing.T, s *space.Space, rel, content string) {
	t.Helper()
	path := filepath.Join(s.DocsDir(), filepath.FromSlash(strings.TrimPrefix(rel, "docs/")))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const sampleDoc = `---
title: 测试文档
author: Eman
date: 2026-08-13
tag: Plainship
---

# Hello

正文内容.
`

func TestBuild_Basic(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/测试文档.md", sampleDoc)

	res, err := Build(s, os.Stdout)
	if err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	if res.ChangedPages != 1 {
		t.Errorf("ChangedPages = %d, 期望 1", res.ChangedPages)
	}
	if !fileExists(t, filepath.Join(s.BuildDir(), "index.html")) {
		t.Error("缺少首页 index.html")
	}
	if !fileExists(t, filepath.Join(s.BuildDir(), "测试文档", "index.html")) {
		t.Error("缺少中文路径文章页")
	}
	if !fileExists(t, filepath.Join(s.BuildDir(), "assets", "app.css")) {
		t.Error("缺少主题资源")
	}
	if !fileExists(t, filepath.Join(s.BuildDir(), "sitemap.xml")) {
		t.Error("缺少 sitemap.xml")
	}
	if !fileExists(t, filepath.Join(s.BuildDir(), "robots.txt")) {
		t.Error("缺少 robots.txt")
	}
}

func TestBuild_Incremental(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/测试文档.md", sampleDoc)

	res1, err := Build(s, nil)
	if err != nil {
		t.Fatalf("首次构建失败: %v", err)
	}
	if res1.ChangedPages != 1 {
		t.Fatalf("首次构建 ChangedPages = %d", res1.ChangedPages)
	}

	// 无变化: 应复用缓存.
	res2, err := Build(s, nil)
	if err != nil {
		t.Fatalf("二次构建失败: %v", err)
	}
	if res2.CopiedPages != 1 {
		t.Errorf("二次构建 CopiedPages = %d, 期望 1", res2.CopiedPages)
	}
	if res2.ChangedPages != 0 {
		t.Errorf("二次构建 ChangedPages = %d, 期望 0", res2.ChangedPages)
	}

	// 修改内容: 只重建 1 页.
	writeDoc(t, s, "docs/测试文档.md", strings.Replace(sampleDoc, "正文内容", "修改后内容", 1))
	res3, err := Build(s, nil)
	if err != nil {
		t.Fatalf("三次构建失败: %v", err)
	}
	if res3.ChangedPages != 1 {
		t.Errorf("三次构建 ChangedPages = %d, 期望 1", res3.ChangedPages)
	}
}

func TestBuild_DeleteHandling(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/a.md", sampleDoc)
	writeDoc(t, s, "docs/b.md", sampleDoc)

	if _, err := Build(s, nil); err != nil {
		t.Fatalf("首次构建失败: %v", err)
	}
	if err := os.Remove(filepath.Join(s.DocsDir(), "a.md")); err != nil {
		t.Fatal(err)
	}
	res, err := Build(s, nil)
	if err != nil {
		t.Fatalf("二次构建失败: %v", err)
	}
	if res.DeletedPages != 1 {
		t.Errorf("DeletedPages = %d, 期望 1", res.DeletedPages)
	}
	// build 中删除的文档应被清理.
	if fileExists(t, filepath.Join(s.BuildDir(), "a", "index.html")) {
		t.Error("已删除文档的产物仍存在于 build")
	}
	if !fileExists(t, filepath.Join(s.BuildDir(), "b", "index.html")) {
		t.Error("未删除文档的产物丢失")
	}
}

func TestBuild_ThemeChangeForcesRebuild(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/测试文档.md", sampleDoc)

	if _, err := Build(s, nil); err != nil {
		t.Fatalf("首次构建失败: %v", err)
	}
	// 修改主题资源.
	cssPath := filepath.Join(s.ThemesDir(), "default", "assets", "app.css")
	f, err := os.OpenFile(cssPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n/* 测试变更 */\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	res, err := Build(s, nil)
	if err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	if res.CopiedPages != 0 {
		t.Errorf("主题变化后不应复用缓存, CopiedPages = %d", res.CopiedPages)
	}
	if res.ChangedPages != 1 {
		t.Errorf("主题变化后应重建, ChangedPages = %d", res.ChangedPages)
	}
}

func TestBuild_ConfigChangeForcesRebuild(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/测试文档.md", sampleDoc)

	if _, err := Build(s, nil); err != nil {
		t.Fatalf("首次构建失败: %v", err)
	}
	// 修改配置标题.
	_ = s.Config.SpaceSite.SiteTitle.Set("新标题")
	res, err := Build(s, nil)
	if err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	if res.ChangedPages != 1 {
		t.Errorf("配置变化后应重建, ChangedPages = %d", res.ChangedPages)
	}
	home, err := os.ReadFile(filepath.Join(s.BuildDir(), "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(home), "新标题") {
		t.Error("首页未使用新配置标题")
	}
}

func TestBuild_SlugAndNested(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/guide/快速开始.md", `---
title: 快速开始
slug: quick-start
---

内容
`)
	res, err := Build(s, nil)
	if err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	if res.ChangedPages != 1 {
		t.Errorf("ChangedPages = %d", res.ChangedPages)
	}
	if !fileExists(t, filepath.Join(s.BuildDir(), "guide", "quick-start", "index.html")) {
		t.Error("slug 输出路径不正确")
	}
	if !fileExists(t, filepath.Join(s.BuildDir(), "guide", "index.html")) {
		t.Error("缺少目录列表页")
	}
}

// TestBuild_RouteConflict 验证同 slug 的两篇文档导致构建失败 (不再静默覆盖).
func TestBuild_RouteConflict(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/a.md", "---\ntitle: A\nslug: same\n---\n内容")
	writeDoc(t, s, "docs/b.md", "---\ntitle: B\nslug: same\n---\n内容")

	_, err := Build(s, nil)
	if err == nil {
		t.Fatal("同 slug 的两篇文档应导致构建失败")
	}
	if !strings.Contains(err.Error(), "路由冲突") && !strings.Contains(err.Error(), "route conflict") {
		t.Errorf("错误信息不明确: %v", err)
	}
}

// TestBuild_InvalidSlugFails 验证非法 slug (路径穿越) 导致构建失败.
func TestBuild_InvalidSlugFails(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/a.md", "---\ntitle: A\nslug: ../evil\n---\n内容")

	if _, err := Build(s, nil); err == nil {
		t.Fatal("非法 slug 应导致构建失败")
	}
	// build 目录外不应产生任何文件.
	if _, err := os.Stat(filepath.Join(filepath.Dir(s.BuildDir()), "evil", "index.html")); err == nil {
		t.Error("非法 slug 不应在 build 目录外写文件")
	}
}

// TestBuild_IndexDocOwnsDir 验证 docs/<dir>/index.md 占据 <dir>/ 路由,
// 且不再生成自动列表页覆盖它.
func TestBuild_IndexDocOwnsDir(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/guide/index.md", "---\ntitle: 指南首页\n---\n索引内容")
	writeDoc(t, s, "docs/guide/foo.md", "---\ntitle: Foo\n---\n内容")

	if _, err := Build(s, nil); err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	// guide/ 由 index.md 占据.
	html, err := os.ReadFile(filepath.Join(s.BuildDir(), "guide", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), "索引内容") {
		t.Errorf("guide/ 应为 index.md 的页面: %s", string(html))
	}
	// 其他文档仍在 guide/ 下, 首页可链接到它.
	if !fileExists(t, filepath.Join(s.BuildDir(), "guide", "foo", "index.html")) {
		t.Error("guide/foo 页面缺失")
	}
}

// TestBuild_HiddenDirSkipped 验证 docs 下隐藏目录 (如 .git) 不会作为资源发布.
func TestBuild_HiddenDirSkipped(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/测试文档.md", sampleDoc)
	writeDoc(t, s, "docs/.git/config", "[core]")

	if _, err := Build(s, nil); err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	if fileExists(t, filepath.Join(s.BuildDir(), "docs", ".git", "config")) {
		t.Error("隐藏目录内容不应被发布")
	}
}

// TestBuild_MarkdownUnsafeConfig 验证 markdown.unsafe 配置生效.
func TestBuild_MarkdownUnsafeConfig(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/a.md", "---\ntitle: A\n---\n正文 <script>x</script>")

	// 默认: 转义.
	if _, err := Build(s, nil); err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	html, _ := os.ReadFile(filepath.Join(s.BuildDir(), "a", "index.html"))
	if strings.Contains(string(html), "<script>x</script>") {
		t.Error("默认 unsafe=false 时不应直通 script")
	}

	// 开启 unsafe.
	_ = s.Config.SpaceSite.MarkdownUnsafe.Set(true)
	if _, err := Build(s, nil); err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	html2, _ := os.ReadFile(filepath.Join(s.BuildDir(), "a", "index.html"))
	if !strings.Contains(string(html2), "<script>x</script>") {
		t.Error("unsafe=true 时应直通 script")
	}
}

func TestBuild_DraftNotPublished(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/草稿.md", "---\ntitle: 草稿\ndraft: true\n---\n内容")
	writeDoc(t, s, "docs/正式.md", "---\ntitle: 正式\n---\n内容")

	if _, err := Build(s, nil); err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	if fileExists(t, filepath.Join(s.BuildDir(), "草稿", "index.html")) {
		t.Error("草稿不应发布")
	}
	if !fileExists(t, filepath.Join(s.BuildDir(), "正式", "index.html")) {
		t.Error("正式文档应发布")
	}
}

func TestBuild_ContentAssets(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/测试文档.md", sampleDoc)
	// 内容资源.
	writeDoc(t, s, "docs/images/logo.svg", "<svg></svg>")

	if _, err := Build(s, nil); err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	if !fileExists(t, filepath.Join(s.BuildDir(), "docs", "images", "logo.svg")) {
		t.Error("内容资源未复制到 build")
	}
}

func TestBuild_MarkdownLinksResolved(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/a.md", "---\ntitle: A\n---\n[去看 B](./b.md)")
	writeDoc(t, s, "docs/b.md", "---\ntitle: B\n---\n内容")

	if _, err := Build(s, nil); err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	html, err := os.ReadFile(filepath.Join(s.BuildDir(), "a", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("HTML 内容: %s", string(html))
	if strings.Contains(string(html), "./b.md") {
		t.Error("Markdown 链接未被解析: 仍包含 b.md")
	}
	if !strings.Contains(string(html), `href="/b/"`) {
		t.Error("Markdown 链接未解析为根相对路由 /b/")
	}
}

func TestBuild_PrevNextLinks(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/a.md", "---\ntitle: A\ndate: 2026-01-01\n---\n内容")
	writeDoc(t, s, "docs/b.md", "---\ntitle: B\ndate: 2026-01-02\n---\n内容")
	writeDoc(t, s, "docs/guide/c.md", "---\ntitle: C\ndate: 2026-01-03\n---\n内容")

	if _, err := Build(s, nil); err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(s.BuildDir(), filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	// a 最早: 无上一篇, 下一篇是 b.
	a := read("a/index.html")
	if !strings.Contains(a, `href="/b/"`) {
		t.Errorf("a 的下一篇链接错误: %s", a)
	}
	// c 最新: 上一篇是 b, 无下一篇. b 在 docs 根目录, 路由为 b/.
	c := read("guide/c/index.html")
	if !strings.Contains(c, `href="/b/"`) {
		t.Errorf("c 的上一篇链接错误: %s", c)
	}
	// b 同时有上/下一篇, 且指向正确路由 (带目录前缀).
	b := read("b/index.html")
	if !strings.Contains(b, `href="/a/"`) || !strings.Contains(b, `href="/guide/c/"`) {
		t.Errorf("b 的上/下一篇链接错误: %s", b)
	}
	// 不应残留裸相对链接 (如 href="b/), 否则从文章页跳转会 404.
	for _, html := range []string{a, b, c} {
		if strings.Contains(html, `href="guide/`) || strings.Contains(html, `href="a/`) || strings.Contains(html, `href="b/`) {
			t.Errorf("存在裸相对链接: %s", html)
		}
	}
	// 首页与目录列表页同样使用根相对链接.
	home := read("index.html")
	if !strings.Contains(home, `href="/guide/c/"`) {
		t.Errorf("首页链接错误: %s", home)
	}
	list := read("guide/index.html")
	if !strings.Contains(list, `href="/guide/c/"`) {
		t.Errorf("目录列表页链接错误: %s", list)
	}
}

func TestBuild_PrevNextRefreshOnIncremental(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/a.md", "---\ntitle: A\ndate: 2026-01-01\n---\n内容")
	writeDoc(t, s, "docs/b.md", "---\ntitle: B\ndate: 2026-01-02\n---\n内容")
	if _, err := Build(s, nil); err != nil {
		t.Fatalf("首次构建失败: %v", err)
	}

	// 新增 c: b 的下一篇从"无"变为 c, 即使 b 内容未变化也必须重新渲染.
	writeDoc(t, s, "docs/c.md", "---\ntitle: C\ndate: 2026-01-03\n---\n内容")
	res, err := Build(s, nil)
	if err != nil {
		t.Fatalf("二次构建失败: %v", err)
	}
	if res.ChangedPages != 2 {
		t.Errorf("ChangedPages = %d, 期望 2 (b 关联刷新 + c 新增)", res.ChangedPages)
	}
	if res.CopiedPages != 1 {
		t.Errorf("CopiedPages = %d, 期望 1 (仅 a 可复用)", res.CopiedPages)
	}
	b, err := os.ReadFile(filepath.Join(s.BuildDir(), "b", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `href="/c/"`) {
		t.Error("b 的下一篇链接未随新增文档刷新")
	}
	// 关联刷新走的是重新渲染: 正文必须完整保留 (状态缓存只有轻量信息).
	if !strings.Contains(string(b), "<p>内容</p>") {
		t.Error("b 关联刷新后正文丢失")
	}

	// 再次构建: 关联稳定, 全部复用缓存.
	res2, err := Build(s, nil)
	if err != nil {
		t.Fatalf("三次构建失败: %v", err)
	}
	if res2.CopiedPages != 3 {
		t.Errorf("CopiedPages = %d, 期望 3", res2.CopiedPages)
	}
	if res2.ChangedPages != 0 {
		t.Errorf("ChangedPages = %d, 期望 0", res2.ChangedPages)
	}

	// 删除 c: b 的下一篇应回到"无", b 需要重新渲染.
	if err := os.Remove(filepath.Join(s.DocsDir(), "c.md")); err != nil {
		t.Fatal(err)
	}
	res3, err := Build(s, nil)
	if err != nil {
		t.Fatalf("四次构建失败: %v", err)
	}
	if res3.ChangedPages != 1 {
		t.Errorf("删除后 ChangedPages = %d, 期望 1 (b 关联刷新)", res3.ChangedPages)
	}
	b2, err := os.ReadFile(filepath.Join(s.BuildDir(), "b", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b2), `href="/c/"`) {
		t.Error("b 的下一篇链接未随删除文档刷新")
	}
}

func TestBuild_BasePathAndDev(t *testing.T) {
	s := setupSpace(t)
	_ = s.Config.SpaceSite.SiteURL.Set("https://example.com/blog")
	writeDoc(t, s, "docs/a.md", "---\ntitle: A\ndate: 2026-01-01\n---\n[去 B](./b.md)")
	writeDoc(t, s, "docs/b.md", "---\ntitle: B\ndate: 2026-01-02\n---\n内容")

	// 生产构建: 链接带基础路径前缀 /blog.
	if _, err := Build(s, nil); err != nil {
		t.Fatalf("生产构建失败: %v", err)
	}
	a, err := os.ReadFile(filepath.Join(s.BuildDir(), "a", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(a), `href="/blog/b/"`) {
		t.Errorf("生产构建的链接应带基础路径: %s", string(a))
	}
	home, err := os.ReadFile(filepath.Join(s.BuildDir(), "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(home), `href="/blog/a/"`) {
		t.Error("首页链接应带基础路径")
	}

	// dev 构建: 链接使用根路径, 与 dev 服务器一致.
	if _, err := BuildDev(s, nil); err != nil {
		t.Fatalf("dev 构建失败: %v", err)
	}
	a2, err := os.ReadFile(filepath.Join(s.BuildDir(), "a", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(a2), `href="/b/"`) {
		t.Errorf("dev 构建链接应为根路径: %s", string(a2))
	}
	if strings.Contains(string(a2), "/blog/") {
		t.Error("dev 构建不应包含基础路径")
	}

	// dev 之后切回生产构建: 基础路径变化必须触发全量重建.
	res, err := Build(s, nil)
	if err != nil {
		t.Fatalf("切回生产构建失败: %v", err)
	}
	if res.CopiedPages != 0 {
		t.Errorf("基础路径变化后不应复用缓存, CopiedPages = %d", res.CopiedPages)
	}
	if res.ChangedPages != 2 {
		t.Errorf("基础路径变化后应重建全部页面, ChangedPages = %d", res.ChangedPages)
	}
}

func TestBuild_AssetLinksResolved(t *testing.T) {
	s := setupSpace(t)
	writeDoc(t, s, "docs/guide/foo.md", "---\ntitle: A\n---\n![logo](images/logo.svg)")
	writeDoc(t, s, "docs/guide/images/logo.svg", "<svg></svg>")

	if _, err := Build(s, nil); err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	html, err := os.ReadFile(filepath.Join(s.BuildDir(), "guide", "foo", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `src="/docs/guide/images/logo.svg"`) {
		t.Errorf("图片链接未解析为资源 URL: %s", string(html))
	}
	if strings.Contains(string(html), `src="images/logo.svg"`) {
		t.Error("图片链接仍为相对地址")
	}
}

func fileExists(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
