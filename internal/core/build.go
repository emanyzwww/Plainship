// Package core 是 Plainship 的核心编排层.
//
// 只负责流程编排 CreateSpace / Build / Publish / Status / Dev;
// Git 语义, 类别划分 / 指纹 / 提交协议 / 编号, 由 `internal/revision` 提供.
package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/emanyzwww/plainship/internal/builder"
	"github.com/emanyzwww/plainship/internal/fsutil"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/layout"
	"github.com/emanyzwww/plainship/internal/revision"
	"github.com/emanyzwww/plainship/internal/space"
	"github.com/emanyzwww/plainship/internal/state"
	"github.com/emanyzwww/plainship/internal/ui"
	"github.com/emanyzwww/plainship/internal/version"
)

// BuildResult 是一次 build 的结果.
type BuildResult struct {
	Build       *builder.Result // Build 是构建结果.
	BuildNumber string          // BuildNumber 是构建编号.
	Commits     []string        // Commits 是本次提交的消息列表.
}

// BuildOptions 控制 Build 行为.
type BuildOptions struct {
	Message string // Message 是可选的 docs 提交备注, 写入提交 body, 不影响机器解析.
}

// Build 执行完整构建流程: 检测变更 -> 构建 -> 分步提交 -> 打编号.
//
// 构建失败时不会创建任何提交与编号.
//
// out 是输出入口, nil 表示静默.
func Build(spaceRoot string, opts BuildOptions, out ui.UI) (*BuildResult, error) {
	if out == nil {
		out = ui.Discard
	}
	s, err := space.Load(spaceRoot)
	if err != nil {
		return nil, err
	}
	out.Info(ui.Bold(fmt.Sprintf("Plainship v%s", version.Version)))

	// Git 是 build 的前置条件, 提交与编号依赖 Git.
	if !s.GitAvailable || !revision.IsRepo(s) {
		return nil, i18n.Errorf(i18n.CoreBuildNeedGit)
	}
	gs := revision.GitStatus(s)
	// Branch 与类别变更统计, 经 Detail + Table 输出.
	out.Detail(i18n.T(i18n.CoreBuildBranchLabel), gs.Branch)
	var rows [][]string
	for _, cat := range revision.Categories {
		c := gs.Changes[cat]
		if c.HasChanges() {
			var parts []string
			if c.Added > 0 {
				parts = append(parts, i18n.T(i18n.CoreBuildAdded, c.Added))
			}
			if c.Modified > 0 {
				parts = append(parts, i18n.T(i18n.CoreBuildModified, c.Modified))
			}
			if c.Deleted > 0 {
				parts = append(parts, i18n.T(i18n.CoreBuildDeleted, c.Deleted))
			}
			rows = append(rows, []string{string(cat), strings.Join(parts, "  ")})
		} else {
			rows = append(rows, []string{string(cat), ui.Green(i18n.T(i18n.CoreBuildCleanCell))})
		}
	}
	out.Table(nil, rows)
	out.Info("")

	// 计算类别指纹, 用作提交信息与 publish 守卫的依据.
	hashes := map[revision.Category]string{}
	for _, cat := range revision.Categories {
		h, err := revision.CategoryHash(s, cat)
		if err != nil {
			return nil, err
		}
		hashes[cat] = h
	}

	// 无变更且已构建过, 且基础路径与渲染器版本与当前一致 -> 直接返回.
	prev, _ := state.LoadState(s.Root)
	if !gs.Changes[revision.CategoryConfig].HasChanges() &&
		!gs.Changes[revision.CategoryTheme].HasChanges() &&
		!gs.Changes[revision.CategoryDocs].HasChanges() &&
		prev.LastBuildID != "" && fsutil.IsDir(s.BuildDir()) &&
		prev.BasePath == builder.BasePath(s, false) &&
		prev.RendererVersion == version.RendererVersion() {
		out.Success(i18n.T(i18n.CoreBuildNoChanges, orEmpty(prev.BuildNumber)))
		return &BuildResult{BuildNumber: prev.BuildNumber}, nil
	}

	// 构建, 失败则不提交也不打号.
	// 进度提示由 builder 内部输出, 此处不重复.
	buildRes, err := builder.Build(s, out)
	if err != nil {
		return nil, err
	}
	out.Info("")
	out.Info(ui.Bold(i18n.T(i18n.CoreBuildComplete)))
	out.Summary(
		i18n.T(i18n.CoreBuildChangedPart, buildRes.ChangedPages),
		i18n.T(i18n.CoreBuildCopiedPart, buildRes.CopiedPages),
		i18n.T(i18n.CoreBuildDeletedPart, buildRes.DeletedPages),
	)

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
		// 样张格式: ✓ commit <类别> build=<编号> <短哈希>, hash= 前缀省略.
		subject := strings.Replace(strings.SplitN(msg, "\n", 2)[0], " hash=", " ", 1)
		out.Success(i18n.T(i18n.CoreBuildCommitLine, subject))
	}

	// 打编号 tag.
	if err := revision.TagBuild(s.Root, num); err != nil {
		return nil, err
	}
	// 记录状态: 编号 + 类别指纹, 在提交与 tag 全部成功之后保存.
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
	out.Success(i18n.T(i18n.CoreBuildTagLine, num))
	out.Info("")
	out.Detail(i18n.T(i18n.CoreBuildReleaseLabel), ui.Cyan(num)+"  ·  "+time.Now().Format("2006-01-02 15:04:05"))
	out.Flush()
	return &BuildResult{Build: buildRes, BuildNumber: num, Commits: commits}, nil
}
