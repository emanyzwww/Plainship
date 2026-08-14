// Package builder 实现 Plainship 构建管线.
// 职责: 扫描, 变化检测, 解析, 渲染, 生成索引与 SEO, 原子发布, 生成清单.
// 本包不直接调用 Git, 变化检测基于内容哈希与构建状态, Git 状态由上层展示.
package builder

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/emanyzwww/Plainship/internal/fsutil"
	"github.com/emanyzwww/Plainship/internal/hash"
	"github.com/emanyzwww/Plainship/internal/i18n"
	"github.com/emanyzwww/Plainship/internal/layout"
	"github.com/emanyzwww/Plainship/internal/manifest"
	"github.com/emanyzwww/Plainship/internal/model"
	"github.com/emanyzwww/Plainship/internal/parser"
	"github.com/emanyzwww/Plainship/internal/router"
	"github.com/emanyzwww/Plainship/internal/space"
	"github.com/emanyzwww/Plainship/internal/state"
	"github.com/emanyzwww/Plainship/internal/theme"
	"github.com/emanyzwww/Plainship/internal/version"
)

// 渲染器版本统一由 internal/version 包提供 (跟随产品版本).

// Build 执行生产构建: 站点内链接基于 site.url 的基础路径 (见 BasePath).
// 开发模式请使用 BuildDev.
func Build(s *space.Space, out io.Writer) (*Result, error) {
	return build(s, out, false)
}

// BuildDev 执行开发模式构建: 与 Build 相同, 但链接使用根路径,
// 与 dev 服务器 (挂在根路径) 保持一致.
func BuildDev(s *space.Space, out io.Writer) (*Result, error) {
	return build(s, out, true)
}

// BasePath 返回构建使用的链接基础路径.
// dev 模式始终返回空字符串 (dev 服务器挂在根路径);
// 生产模式返回 site.url 的路径部分, 例如 https://example.com/repo -> /repo;
// 站点部署在域名根路径或未配置 url 时返回空字符串.
func BasePath(s *space.Space, dev bool) string {
	if dev {
		return ""
	}
	u, err := url.Parse(strings.TrimRight(s.Config.Site.URL, "/"))
	if err != nil || u.Host == "" || u.Path == "" || u.Path == "/" {
		return ""
	}
	return strings.TrimRight(u.Path, "/")
}

// Result 是一次构建的结果.
type Result struct {
	BuildID      string
	ChangedPages int
	CopiedPages  int
	DeletedPages int
	AssetFiles   int
	TotalFiles   int
	BuildPath    string
	Manifest     *manifest.Manifest
}

// build 执行完整构建流程. dev 为 true 时链接使用根路径.
func build(s *space.Space, out io.Writer, dev bool) (*Result, error) {
	log := func(format string, args ...any) {
		if out != nil {
			fmt.Fprintf(out, format+"\n", args...)
		}
	}
	log(i18n.T(i18n.BuilderScanning))

	// 1. 准备状态与构建输入.
	if err := state.EnsureDirs(s.Root); err != nil {
		return nil, err
	}
	prevState, err := state.LoadState(s.Root)
	if err != nil {
		return nil, err
	}
	cfgHash, err := s.Config.Hash()
	if err != nil {
		return nil, err
	}
	siteLang := i18n.Parse(s.Config.Site.Language)
	base := BasePath(s, dev)
	themeObj, err := theme.Load(s.Root, s.Config.Theme.Name, siteLang, base)
	if err != nil {
		return nil, err
	}
	themeHash, err := themeDirHash(s.Root, s.Config.Theme.Name, themeObj)
	if err != nil {
		return nil, err
	}
	// 全局输入变化时, 所有页面都需要重新构建.
	// 基础路径变化 (如 dev 构建后切回生产构建) 同样触发全量重建.
	inputsChanged := prevState.ConfigHash != cfgHash ||
		prevState.ThemeHash != themeHash ||
		prevState.RendererVersion != version.RendererVersion() ||
		prevState.BasePath != base

	// 2. 扫描 docs 目录.
	mdFiles, assetFiles, err := scanContents(s.DocsDir())
	if err != nil {
		return nil, i18n.Errorf(i18n.BuilderScanFail, err)
	}

	// 3. 计算每个文件的哈希并分类.
	// kind: added / modified / unchanged / draft.
	changes := map[string]string{}
	for _, srcRel := range mdFiles {
		abs := filepath.Join(s.DocsDir(), filepath.FromSlash(strings.TrimPrefix(srcRel, layout.DocsDir+"/")))
		h, err := hash.File(abs)
		if err != nil {
			return nil, i18n.Errorf(i18n.BuilderHashFail, srcRel, err)
		}
		prev, ok := prevState.Files[srcRel]
		if !ok {
			changes[srcRel] = "added"
		} else if inputsChanged || prev.Hash != h {
			changes[srcRel] = "modified"
		} else {
			changes[srcRel] = "unchanged"
		}
	}

	// 4. 第一遍: 预解析路由, 供链接解析使用.
	resolver := router.NewWithBase(base)
	// 登记内容资源, 供 Markdown 非 md 链接 (图片等) 解析.
	for _, assetRel := range assetFiles {
		resolver.RegisterAsset(assetRel, assetRel)
	}
	// 路由冲突检测: 两篇文档生成同一路由时构建失败, 避免静默覆盖.
	routeOwners := map[string]string{}
	for _, srcRel := range mdFiles {
		kind := changes[srcRel]
		var route string
		if kind == "unchanged" {
			route = prevState.Files[srcRel].Route
		} else {
			meta, _, err := parser.SplitFrontMatterFile(filepath.Join(s.DocsDir(), relToAbs(s.DocsDir(), srcRel)))
			if err != nil {
				return nil, i18n.Errorf(i18n.BuilderFmFail, srcRel, err)
			}
			var err2 error
			route, _, err2 = router.RouteFor(srcRel, meta)
			if err2 != nil {
				return nil, i18n.Errorf(i18n.BuilderRouteFail, srcRel, err2)
			}
		}
		if route != "" {
			if owner, dup := routeOwners[route]; dup && owner != srcRel {
				return nil, i18n.Errorf(i18n.BuilderRouteConflict, owner, srcRel, route)
			}
			routeOwners[route] = srcRel
			resolver.Register(srcRel, route)
		}
	}

	// 5. 创建构建目录.
	// buildID 使用纳秒时间戳, 保证同一秒内多次构建也互不冲突.
	buildID := "build-" + time.Now().Format("20060102-150405") + "-" + hash.BuildID(fmt.Sprintf("%d-%s", time.Now().UnixNano(), strings.Join(mdFiles, "|")))
	buildDir := state.BuildDir(s.Root, buildID)
	if err := os.RemoveAll(buildDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return nil, err
	}
	prevBuildDir := ""
	if prevState.LastBuildID != "" {
		prevBuildDir = state.BuildDir(s.Root, prevState.LastBuildID)
		if !fsutil.Exists(prevBuildDir) {
			prevBuildDir = "" // 上一构建目录已被清理, 无法复用缓存.
		}
	}

	res := &Result{BuildID: buildID, BuildPath: s.BuildDir()}
	site := model.Site{
		Title:       s.Config.Site.Title,
		Description: s.Config.Site.Description,
		URL:         strings.TrimRight(s.Config.Site.URL, "/"),
		Language:    s.Config.Site.Language,
		BaseURL:     base,
	}
	// 6. 解析文档 (仅完整解析发生变化的文档, 未变化的复用状态缓存).
	log(i18n.T(i18n.BuilderBuilding))
	sortedMd := append([]string(nil), mdFiles...)
	sort.Strings(sortedMd)
	parsed := map[string]*model.Document{}
	unchangedDocs := map[string]*model.Document{}
	newStateFiles := map[string]state.FileState{}
	manifestInstance := manifest.New(buildID, s.Config.Site.SiteID, hash.Inputs(map[string]string{
		"config": cfgHash, "theme": themeHash, "renderer": version.RendererVersion(),
	}))

	for _, srcRel := range sortedMd {
		kind := changes[srcRel]
		abs := filepath.Join(s.DocsDir(), relToAbs(s.DocsDir(), srcRel))
		content, err := os.ReadFile(abs)
		if err != nil {
			return nil, i18n.Errorf(i18n.BuilderReadFail, srcRel, err)
		}
		if kind == "unchanged" {
			// 未变化: 使用状态缓存构造轻量文档, 用于列表与链接解析.
			fs := prevState.Files[srcRel]
			doc := &model.Document{
				SourcePath: srcRel,
				FileName:   filepath.Base(srcRel),
				Stem:       strings.TrimSuffix(filepath.Base(srcRel), filepath.Ext(srcRel)),
				Title:      fs.Title, Date: fs.Date, Summary: fs.Summary,
				Route: fs.Route, OutputPath: fs.Output,
				Meta: model.Metadata{}, Hash: fs.Hash,
			}
			unchangedDocs[srcRel] = doc
			newStateFiles[srcRel] = fs
			manifestInstance.Add(manifest.FileEntry{Source: srcRel, Output: fs.Output, Hash: fs.Hash, Type: manifest.TypePage})
			continue
		}
		doc, err := parser.Parse(content, srcRel,
			parser.WithLinkResolver(resolver.ResolveLink),
			parser.WithUnsafe(s.Config.Markdown.Unsafe))
		if err != nil {
			return nil, err
		}
		if route, ok := resolver.Lookup(srcRel); ok {
			doc.Route = route
			doc.OutputPath = route + "index.html"
		} else {
			var err2 error
			doc.Route, doc.OutputPath, err2 = router.RouteFor(srcRel, doc.Meta)
			if err2 != nil {
				return nil, i18n.Errorf(i18n.BuilderRouteFail, srcRel, err2)
			}
		}
		doc.Hash = hash.String(string(content))

		// 草稿不发布.
		if doc.Draft {
			if _, existed := prevState.Files[srcRel]; existed {
				res.DeletedPages++
				prev := prevState.Files[srcRel]
				manifestInstance.AddDeleted(manifest.FileEntry{Source: srcRel, Output: prev.Output, Type: manifest.TypePage})
				log(i18n.T(i18n.BuilderDraft, srcRel))
			}
			continue
		}

		parsed[srcRel] = doc
		newStateFiles[srcRel] = state.FileState{
			Hash: doc.Hash, Output: doc.OutputPath, Route: doc.Route,
			Type: "page", Title: doc.Title, Date: doc.Date, Summary: doc.Summary,
		}
		manifestInstance.Add(manifest.FileEntry{Source: srcRel, Output: doc.OutputPath, Hash: doc.Hash, Type: manifest.TypePage})
	}

	// 7. 处理删除的文档与资源.
	for srcRel, fs := range prevState.Files {
		switch fs.Type {
		case "page":
			if !containsKey(mdFiles, srcRel) {
				res.DeletedPages++
				manifestInstance.AddDeleted(manifest.FileEntry{Source: srcRel, Output: fs.Output, Hash: fs.Hash, Type: manifest.TypePage})
				log(i18n.T(i18n.BuilderDeleted, srcRel))
			}
		case "asset":
			if !containsKey(assetFiles, srcRel) {
				manifestInstance.AddDeleted(manifest.FileEntry{Source: srcRel, Output: fs.Output, Hash: fs.Hash, Type: manifest.TypeAsset})
			}
		}
	}

	// 8. 生成列表信息并计算上一篇 / 下一篇.
	allInfo := collectDocInfos(parsed, unchangedDocs)
	sortDocs(allInfo, s.Config.List.Sort)
	prevNext := computePrevNext(allInfo)

	// 记录每篇文档的上一篇/下一篇路由到状态, 供下次增量构建检测关联变化.
	for srcRel, doc := range parsed {
		if fs, ok := newStateFiles[srcRel]; ok {
			pn := prevNext[doc.Route]
			fs.PrevRoute = pn.prevRoute()
			fs.NextRoute = pn.nextRoute()
			newStateFiles[srcRel] = fs
		}
	}

	// 9. 输出文档页面.
	for _, srcRel := range sortedMd {
		doc, ok := parsed[srcRel]
		if !ok {
			continue
		}
		// 增量: 内容未变化且全局输入(主题/配置/渲染器)未变时复用上一构建产物.
		// 注意: 即使内容未变化, 上一篇/下一篇关联变化时也必须重新渲染.
		prevFS, hadPrev := prevState.Files[srcRel]
		pn := prevNext[doc.Route]
		assocChanged := !hadPrev || prevFS.PrevRoute != pn.prevRoute() || prevFS.NextRoute != pn.nextRoute()
		if !inputsChanged && !assocChanged && hadPrev && prevFS.Hash == doc.Hash && prevBuildDir != "" {
			prevOutput := filepath.Join(prevBuildDir, filepath.FromSlash(doc.OutputPath))
			if fsutil.Exists(prevOutput) {
				dst := filepath.Join(buildDir, filepath.FromSlash(doc.OutputPath))
				if err := fsutil.CopyFile(prevOutput, dst); err != nil {
					return nil, err
				}
				res.CopiedPages++
				continue
			}
		}
		layout := doc.Layout
		if layout == "" || !themeObj.HasLayout(layout) {
			layout = "article"
		}
		data := model.PageData{Site: site, Page: doc, Build: "article", Prev: pn.prev, Next: pn.next}
		htmlOut, err := themeObj.Render(layout, data)
		if err != nil {
			return nil, i18n.Errorf(i18n.BuilderRenderFail, srcRel, err)
		}
		dst := filepath.Join(buildDir, filepath.FromSlash(doc.OutputPath))
		if err := fsutil.WriteFile(dst, []byte(htmlOut)); err != nil {
			return nil, err
		}
		res.ChangedPages++
		log("✓ %s", doc.Title)
	}

	// 输出未变化文档 (从上一构建复制).
	// 注意: 即使内容未变化, 上一篇/下一篇关联变化时 (新增/删除/改日期/改 slug)
	// 也必须重新渲染, 否则文章间的导航地址是陈旧的.
	for _, doc := range unchangedDocs {
		pn := prevNext[doc.Route]
		prevFS := prevState.Files[doc.SourcePath]
		assocChanged := prevFS.PrevRoute != pn.prevRoute() || prevFS.NextRoute != pn.nextRoute()
		if prevBuildDir != "" && !assocChanged {
			src := filepath.Join(prevBuildDir, filepath.FromSlash(doc.OutputPath))
			if fsutil.Exists(src) {
				dst := filepath.Join(buildDir, filepath.FromSlash(doc.OutputPath))
				if err := fsutil.CopyFile(src, dst); err != nil {
					return nil, err
				}
				res.CopiedPages++
				continue
			}
		}
		// 缓存不可用或关联已变化: 完整重新解析并渲染.
		// 状态中的缓存只有轻量信息 (无正文), 不能直接用于渲染.
		content, err := os.ReadFile(filepath.Join(s.DocsDir(), relToAbs(s.DocsDir(), doc.SourcePath)))
		if err != nil {
			return nil, err
		}
		full, err := parser.Parse(content, doc.SourcePath,
			parser.WithLinkResolver(resolver.ResolveLink),
			parser.WithUnsafe(s.Config.Markdown.Unsafe))
		if err != nil {
			return nil, err
		}
		layout := full.Layout
		if layout == "" || !themeObj.HasLayout(layout) {
			layout = "article"
		}
		data := model.PageData{Site: site, Page: full, Build: "article", Prev: pn.prev, Next: pn.next}
		htmlOut, err := themeObj.Render(layout, data)
		if err != nil {
			return nil, err
		}
		if err := fsutil.WriteFile(filepath.Join(buildDir, filepath.FromSlash(doc.OutputPath)), []byte(htmlOut)); err != nil {
			return nil, err
		}
		res.ChangedPages++
		// 关联已刷新, 同步更新状态.
		if fs, ok := newStateFiles[doc.SourcePath]; ok {
			fs.PrevRoute = pn.prevRoute()
			fs.NextRoute = pn.nextRoute()
			newStateFiles[doc.SourcePath] = fs
		}
	}

	// 10. 复制内容资源 (图片等非 md 文件).
	contentAssetCount := 0
	for _, assetRel := range assetFiles {
		abs := filepath.Join(s.DocsDir(), relToAbs(s.DocsDir(), assetRel))
		h, err := hash.File(abs)
		if err != nil {
			continue
		}
		dst := filepath.Join(buildDir, filepath.FromSlash(assetRel))
		prevFS, hadPrev := prevState.Files[assetRel]
		if hadPrev && prevFS.Hash == h && prevBuildDir != "" {
			// 未变化, 从上一构建复制.
			src := filepath.Join(prevBuildDir, filepath.FromSlash(assetRel))
			if fsutil.Exists(src) {
				if err := fsutil.CopyFile(src, dst); err != nil {
					return nil, err
				}
			} else {
				if err := fsutil.CopyFile(abs, dst); err != nil {
					return nil, err
				}
			}
		} else {
			if err := fsutil.CopyFile(abs, dst); err != nil {
				return nil, err
			}
		}
		newStateFiles[assetRel] = state.FileState{Hash: h, Output: assetRel, Type: "asset"}
		manifestInstance.Add(manifest.FileEntry{Source: assetRel, Output: assetRel, Hash: h, Type: manifest.TypeAsset})
		contentAssetCount++
	}

	// 11. 复制主题资源.
	themeAssets, err := themeObj.WriteAssets(buildDir)
	if err != nil {
		return nil, err
	}
	for rel, data := range themeObj.Assets {
		manifestInstance.Add(manifest.FileEntry{Output: rel, Hash: hash.Bytes(data), Type: manifest.TypeAsset})
	}
	res.AssetFiles = contentAssetCount + themeAssets

	// 12. 生成首页与列表页.
	homeData := model.PageData{Site: site, Docs: allInfo, Build: "home"}
	homeHTML, err := themeObj.Render("home", homeData)
	if err != nil {
		return nil, err
	}
	if err := fsutil.WriteFile(filepath.Join(buildDir, "index.html"), []byte(homeHTML)); err != nil {
		return nil, err
	}
	manifestInstance.Add(manifest.FileEntry{Output: "index.html", Hash: hash.String(homeHTML), Type: manifest.TypeIndex})
	res.TotalFiles++

	// 目录列表页.
	// 目录已有 index 文档 (docs/<dir>/index.md) 时, 该文档占据 <dir>/ 路由,
	// 不再生成自动列表页, 避免覆盖.
	dirs := groupByDir(allInfo)
	for dir, docs := range dirs {
		if dir == "" || hasDirIndex(allInfo, dir) {
			continue
		}
		listData := model.PageData{Site: site, Docs: docs, Dir: dir, Build: "list"}
		listHTML, err := themeObj.Render("list", listData)
		if err != nil {
			return nil, err
		}
		outRel := dir + "/index.html"
		if err := fsutil.WriteFile(filepath.Join(buildDir, filepath.FromSlash(outRel)), []byte(listHTML)); err != nil {
			return nil, err
		}
		manifestInstance.Add(manifest.FileEntry{Output: outRel, Hash: hash.String(listHTML), Type: manifest.TypeIndex})
		res.TotalFiles++
	}

	// 13. 生成 SEO 文件.
	if err := writeSEO(buildDir, site, allInfo); err != nil {
		return nil, err
	}

	// 14. 校验产物并原子发布.
	if !fsutil.Exists(filepath.Join(buildDir, "index.html")) {
		return nil, i18n.Errorf(i18n.BuilderNoIndex)
	}
	if err := activate(buildDir, s.BuildDir()); err != nil {
		return nil, i18n.Errorf(i18n.BuilderActivateFail, err)
	}

	// 14.1 生产构建压缩产物 (HTML/CSS/JS); dev 构建保持可读.
	// 压缩在激活之后、清单写入之前, 并同步刷新清单哈希.
	if !dev {
		if err := minifyDir(s.BuildDir(), manifestInstance); err != nil {
			return nil, err
		}
	}

	// 13. 写入清单与状态.
	res.Manifest = manifestInstance
	if err := manifest.Write(s.Root, manifestInstance); err != nil {
		return nil, err
	}
	newState := state.NewBuildState()
	newState.LastBuildID = buildID
	newState.RendererVersion = version.RendererVersion()
	newState.ConfigHash = cfgHash
	newState.ThemeHash = themeHash
	newState.BasePath = base
	newState.Files = newStateFiles
	if err := state.SaveState(s.Root, newState); err != nil {
		return nil, err
	}
	res.TotalFiles = len(manifestInstance.Files) + len(manifestInstance.Deleted)
	// 清理旧构建, 保留最近 2 个.
	pruneOldBuilds(s.Root, 2, buildID)
	return res, nil
}

// scanContents 扫描 docs 目录.
// 返回 Markdown 文件与资源文件列表(均为相对 Space 根的路径, 使用正斜杠).
func scanContents(docsDir string) (mdFiles, assetFiles []string, err error) {
	if !fsutil.IsDir(docsDir) {
		return nil, nil, i18n.Errorf(i18n.BuilderNoDocsDir, docsDir)
	}
	err = filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// 跳过隐藏目录 (如 .git / .github), 防止内部文件被当资源发布.
			if path != docsDir && strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(docsDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		base := filepath.Base(rel)
		if strings.HasPrefix(base, ".") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(base))
		switch {
		case ext == ".md" || ext == ".markdown":
			mdFiles = append(mdFiles, layout.DocsDir+"/"+rel)
		default:
			assetFiles = append(assetFiles, layout.DocsDir+"/"+rel)
		}
		return nil
	})
	return mdFiles, assetFiles, err
}

// themeDirHash 计算主题的联合哈希.
// 主题来自 Space 目录时哈希全部主题文件, 否则使用内嵌主题版本.
func themeDirHash(spaceRoot, name string, t *theme.Theme) (string, error) {
	if t.HasOwnFS {
		dir := filepath.Join(spaceRoot, layout.ThemesDir, name)
		inputs := map[string]string{}
		files, err := fsutil.ListFiles(dir)
		if err != nil {
			return "", err
		}
		for _, f := range files {
			h, err := hash.File(filepath.Join(dir, filepath.FromSlash(f)))
			if err != nil {
				return "", err
			}
			inputs[f] = h
		}
		return hash.Inputs(inputs), nil
	}
	return "embedded:" + t.Version, nil
}

// relToAbs 将相对 Space 的路径转换为 docs 目录下的绝对路径.
func relToAbs(docsDir, rel string) string {
	trimmed := strings.TrimPrefix(rel, layout.DocsDir+"/")
	return filepath.FromSlash(trimmed)
}

// collectDocInfos 收集全部文档的列表信息.
// 已解析文档使用解析结果, 未变化文档使用状态中的缓存信息.
func collectDocInfos(parsed map[string]*model.Document, unchanged map[string]*model.Document) []model.DocInfo {
	infos := []model.DocInfo{}
	for srcRel, doc := range parsed {
		infos = append(infos, model.DocInfo{
			Title: doc.Title, Route: doc.Route, Date: doc.Date,
			Tag: doc.Tag, Source: srcRel, Stem: doc.Stem, Summary: doc.Summary,
		})
	}
	for srcRel, doc := range unchanged {
		infos = append(infos, model.DocInfo{
			Title: doc.Title, Route: doc.Route, Date: doc.Date, Source: srcRel, Summary: doc.Summary,
		})
	}
	return infos
}

// prevNextPair 是一篇文档的上一篇与下一篇信息.
type prevNextPair struct {
	prev *model.DocInfo
	next *model.DocInfo
}

// prevRoute 返回上一篇的路由, 无上一篇时返回空字符串.
func (p prevNextPair) prevRoute() string {
	if p.prev != nil {
		return p.prev.Route
	}
	return ""
}

// nextRoute 返回下一篇的路由, 无下一篇时返回空字符串.
func (p prevNextPair) nextRoute() string {
	if p.next != nil {
		return p.next.Route
	}
	return ""
}

// computePrevNext 按日期升序排列文档, 计算每篇的上一篇(较早)与下一篇(较新).
func computePrevNext(allInfo []model.DocInfo) map[string]prevNextPair {
	sorted := append([]model.DocInfo(nil), allInfo...)
	sortDocsByDateAsc(sorted)
	out := map[string]prevNextPair{}
	for i, d := range sorted {
		p := prevNextPair{}
		if i > 0 {
			prev := sorted[i-1]
			p.prev = &prev
		}
		if i < len(sorted)-1 {
			next := sorted[i+1]
			p.next = &next
		}
		out[d.Route] = p
	}
	return out
}

// sortDocsByDateAsc 按日期升序排序, 空日期最后, 同日期按标题升序.
func sortDocsByDateAsc(docs []model.DocInfo) {
	parseDate := func(s string) time.Time {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return time.Time{}
		}
		return t
	}
	sort.SliceStable(docs, func(i, j int) bool {
		ti, tj := parseDate(docs[i].Date), parseDate(docs[j].Date)
		if !ti.Equal(tj) {
			if ti.IsZero() {
				return false
			}
			if tj.IsZero() {
				return true
			}
			return ti.Before(tj)
		}
		return strings.ToLower(docs[i].Title) < strings.ToLower(docs[j].Title)
	})
}

// sortDocs 按配置对文档列表排序.
// 默认 date-desc: 日期新的在前, 日期缺失的排在最后, 同日期按标题升序.
func sortDocs(docs []model.DocInfo, mode string) {
	parseDate := func(s string) time.Time {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return time.Time{}
		}
		return t
	}
	asc := strings.HasSuffix(mode, "asc")
	sort.SliceStable(docs, func(i, j int) bool {
		ti, tj := parseDate(docs[i].Date), parseDate(docs[j].Date)
		if !ti.Equal(tj) {
			if asc {
				return ti.Before(tj)
			}
			// desc: 空日期最后.
			if ti.IsZero() {
				return false
			}
			if tj.IsZero() {
				return true
			}
			return ti.After(tj)
		}
		return strings.ToLower(docs[i].Title) < strings.ToLower(docs[j].Title)
	})
}

// hasDirIndex 判断目录 dir 是否存在索引文档 (路由为 dir/).
func hasDirIndex(docs []model.DocInfo, dir string) bool {
	indexRoute := dir + "/"
	for _, d := range docs {
		if d.Route == indexRoute {
			return true
		}
	}
	return false
}

// groupByDir 将文档按所在目录分组.
func groupByDir(docs []model.DocInfo) map[string][]model.DocInfo {
	groups := map[string][]model.DocInfo{}
	for _, d := range docs {
		dir := dirOfRoute(d.Route)
		groups[dir] = append(groups[dir], d)
	}
	return groups
}

// dirOfRoute 从路由中提取目录, 例如 guide/foo/ -> guide.
func dirOfRoute(route string) string {
	route = strings.Trim(route, "/")
	if route == "" {
		return ""
	}
	idx := strings.LastIndex(route, "/")
	if idx < 0 {
		return ""
	}
	return route[:idx]
}

// writeSEO 生成 sitemap.xml 与 robots.txt.
func writeSEO(buildDir string, site model.Site, docs []model.DocInfo) error {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	base := site.URL
	homeURL := base + "/"
	sb.WriteString(fmt.Sprintf("  <url><loc>%s</loc></url>\n", homeURL))
	for _, d := range docs {
		urlPath := router.EncodePath(d.Route)
		loc := base + "/" + urlPath
		sb.WriteString(fmt.Sprintf("  <url><loc>%s</loc></url>\n", loc))
	}
	sb.WriteString("</urlset>\n")
	if err := fsutil.WriteFile(filepath.Join(buildDir, "sitemap.xml"), []byte(sb.String())); err != nil {
		return err
	}
	robots := "User-agent: *\nAllow: /\n"
	if site.URL != "" {
		robots += "Sitemap: " + base + "/sitemap.xml\n"
	}
	return fsutil.WriteFile(filepath.Join(buildDir, "robots.txt"), []byte(robots))
}

// activate 原子发布 build 目录.
// 先构建到 .plainship/builds, 验证通过后整体发布, 保证旧 build 在失败时保持可用.
// 实现: 先把新内容完整复制到同级临时目录, 再通过两次 rename 交换,
// 失败时旧 build/ 始终保持完整 (不会出现复制到一半的残缺状态).
// 构建目录本身保留在 .plainship/builds, 供后续增量构建复用缓存.
func activate(buildDir, outputDir string) error {
	if !fsutil.Exists(outputDir) {
		return fsutil.CopyDir(buildDir, outputDir)
	}
	ts := time.Now().Format("20060102-150405.000000000")
	tmp := outputDir + ".new-" + ts
	old := outputDir + ".old-" + ts
	// 1. 复制新内容到临时目录 (失败时旧 build 不受影响).
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := fsutil.CopyDir(buildDir, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	// 2. 交换: 旧 build 改名, 新目录就位.
	if err := os.Rename(outputDir, old); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, outputDir); err != nil {
		// 3. 交换失败, 恢复旧 build.
		_ = os.Rename(old, outputDir)
		_ = os.RemoveAll(tmp)
		return err
	}
	return os.RemoveAll(old)
}

// pruneOldBuilds 清理旧的构建目录, 只保留最近 keep 个.
// 用于防止 .plainship/builds 无限膨胀.
func pruneOldBuilds(spaceRoot string, keep int, currentBuildID string) {
	buildsDir := state.BuildsDir(spaceRoot)
	entries, err := os.ReadDir(buildsDir)
	if err != nil {
		return
	}
	type named struct {
		id    string
		mtime int64
	}
	var list []named
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		list = append(list, named{id: e.Name(), mtime: info.ModTime().Unix()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].mtime > list[j].mtime })
	for i, n := range list {
		if i < keep || n.id == currentBuildID {
			continue
		}
		_ = os.RemoveAll(filepath.Join(buildsDir, n.id))
	}
}

func containsKey(list []string, key string) bool {
	for _, s := range list {
		if s == key {
			return true
		}
	}
	return false
}
