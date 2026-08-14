// Package core 是 Plainship 的核心编排层.
// 只负责流程编排 (CreateSpace / Build / Publish / Status / Dev),
// Git 语义 (类别划分, 指纹, 提交协议, 编号) 由 internal/revision 提供.
package core

import (
	"fmt"
	"io"
	"strings"

	"github.com/emanyzwww/Plainship/internal/builder"
	"github.com/emanyzwww/Plainship/internal/fsutil"
	"github.com/emanyzwww/Plainship/internal/i18n"
	"github.com/emanyzwww/Plainship/internal/layout"
	"github.com/emanyzwww/Plainship/internal/revision"
	"github.com/emanyzwww/Plainship/internal/space"
	"github.com/emanyzwww/Plainship/internal/state"
	"github.com/emanyzwww/Plainship/internal/style"
	"github.com/emanyzwww/Plainship/internal/version"
)

// BuildResult 是一次 build 的结果.
type BuildResult struct {
	Build       *builder.Result
	BuildNumber string
	Commits     []string
}

// BuildOptions 控制 Build 行为.
type BuildOptions struct {
	// Message 是可选的 docs 提交备注, 写入提交 body, 不影响机器解析.
	Message string
}

// Build 执行完整构建流程: 检测变更 -> 构建 -> 分步提交 -> 打编号.
// 构建失败时不会创建任何提交与编号.
func Build(spaceRoot string, opts BuildOptions, out io.Writer) (*BuildResult, error) {
	printl := func(format string, args ...any) {
		if out != nil {
			fmt.Fprintf(out, format+"\n", args...)
		}
	}
	// printls 输出已渲染文本 (避免 % 被误当格式符), 用于样式化行.
	printls := func(s string) {
		if out != nil {
			fmt.Fprintln(out, s)
		}
	}
	sty := style.For(out)
	s, err := space.Load(spaceRoot)
	if err != nil {
		return nil, err
	}
	printls(sty.Bold(fmt.Sprintf("Plainship v%s", version.Version)))
	printl("")

	// Git 是 build 的前置条件 (提交与编号依赖 Git).
	if !s.GitAvailable || !revision.IsRepo(s) {
		return nil, i18n.Errorf(i18n.CoreBuildNeedGit)
	}
	gs := revision.GitStatus(s)
	printl(i18n.T(i18n.CoreBuildBranch, gs.Branch))
	for _, cat := range revision.Categories {
		c := gs.Changes[cat]
		if c.HasChanges() {
			printl(i18n.T(i18n.CoreBuildChanges, cat, c.Added, c.Modified, c.Deleted))
		} else {
			printls(sty.Green(i18n.T(i18n.CoreBuildClean, cat)))
		}
	}
	printl("")

	// 计算类别指纹 (提交信息与 publish 守卫的依据).
	hashes := map[revision.Category]string{}
	for _, cat := range revision.Categories {
		h, err := revision.CategoryHash(s, cat)
		if err != nil {
			return nil, err
		}
		hashes[cat] = h
	}

	// 无变更且已构建过 -> 直接返回.
	// 基础路径必须与生产构建一致: dev 构建之后需要重新构建, 避免 dev 链接被发布.
	// 渲染器版本也必须一致: 升级 Plainship 后即使源码无变更也要重建, 避免发布旧渲染.
	prev, _ := state.LoadState(s.Root)
	if !gs.Changes[revision.CategoryConfig].HasChanges() &&
		!gs.Changes[revision.CategoryTheme].HasChanges() &&
		!gs.Changes[revision.CategoryDocs].HasChanges() &&
		prev.LastBuildID != "" && fsutil.IsDir(s.BuildDir()) &&
		prev.BasePath == builder.BasePath(s, false) &&
		prev.RendererVersion == version.RendererVersion() {
		printls(sty.Green(i18n.T(i18n.CoreBuildNoChanges, orEmpty(prev.BuildNumber))))
		return &BuildResult{BuildNumber: prev.BuildNumber}, nil
	}

	// 构建. 失败则不提交、不打号.
	// 进度提示由 builder 内部输出 (BuilderScanning/BuilderBuilding), 此处不重复.
	buildRes, err := builder.Build(s, out)
	if err != nil {
		return nil, err
	}
	printl("")
	printls(sty.Green(i18n.T(i18n.CoreBuildOk, buildRes.ChangedPages, buildRes.CopiedPages, buildRes.DeletedPages)))

	// 计算本次构建编号.
	num, err := revision.NextBuildNumber(s.Root)
	if err != nil {
		return nil, err
	}

	// 分步提交: config -> theme -> docs, 无变更的类别跳过.
	steps := []struct {
		cat   revision.Category
		paths []string
	}{
		{revision.CategoryConfig, []string{layout.ConfigFile, layout.GitignoreFile}},
		{revision.CategoryTheme, []string{layout.ThemesDir}},
		{revision.CategoryDocs, []string{layout.DocsDir}},
	}
	var commits []string
	for _, st := range steps {
		if !gs.Changes[st.cat].HasChanges() {
			continue
		}
		msg := revision.CommitMessage(st.cat, num, hashes[st.cat])
		if st.cat == revision.CategoryDocs && strings.TrimSpace(opts.Message) != "" {
			msg += "\n\n" + strings.TrimSpace(opts.Message)
		}
		if err := revision.CommitPaths(s.Root, msg, st.paths...); err != nil {
			return nil, i18n.Errorf(i18n.CoreBuildCommitFail, st.cat, err)
		}
		commits = append(commits, msg)
		printls(sty.Green(fmt.Sprintf("  ✓ git commit: %s", strings.SplitN(msg, "\n", 2)[0])))
	}

	// 打编号 tag.
	if err := revision.TagBuild(s.Root, num); err != nil {
		return nil, err
	}
	// 记录状态: 编号 + 类别指纹.
	// 在提交与 tag 全部成功之后才保存, 避免失败时状态与 Git 历史不一致.
	bs, _ := state.LoadState(s.Root)
	bs.BuildNumber = num
	bs.CategoryHashes = map[string]string{
		string(revision.CategoryConfig): hashes[revision.CategoryConfig],
		string(revision.CategoryTheme):  hashes[revision.CategoryTheme],
		string(revision.CategoryDocs):   hashes[revision.CategoryDocs],
	}
	if err := state.SaveState(s.Root, bs); err != nil {
		return nil, err
	}
	printls(sty.Cyan(i18n.T(i18n.CoreBuildNumber, num)))
	printl("")
	printl(i18n.T(i18n.CoreBuildDone))
	return &BuildResult{Build: buildRes, BuildNumber: num, Commits: commits}, nil
}
