package core

import (
	"fmt"
	"os"

	"github.com/emanyzwww/plainship/internal/builder"
	"github.com/emanyzwww/plainship/internal/fsutil"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/manifest"
	"github.com/emanyzwww/plainship/internal/protocol"
	"github.com/emanyzwww/plainship/internal/revision"
	"github.com/emanyzwww/plainship/internal/space"
	"github.com/emanyzwww/plainship/internal/state"
	"github.com/emanyzwww/plainship/internal/sync"
	"github.com/emanyzwww/plainship/internal/ui"
	"github.com/emanyzwww/plainship/internal/version"
)

// PublishResult 是一次 publish 的结果.
type PublishResult struct {
	Response *protocol.Response // Response 是服务器响应.
}

// Publish 发布到服务器, 只发布由已提交源码构建出的 `build/` 内容.
//
// 前置守卫:
//  1. 当前源码无未提交变更, config/theme/docs 均 clean.
//  2. `build/` 必须由当前源码构建, 状态中的类别指纹一致.
//
// 输出, 样张 6.2: 版本头 → Checking publish guards 逐项打勾 → 网络阶段
// spinner, 含文件数/字节数, → Uploaded / Activated 结果行.
func Publish(spaceRoot string, out ui.UI) (*PublishResult, error) {
	if out == nil {
		out = ui.Discard
	}
	s, err := space.Load(spaceRoot)
	if err != nil {
		return nil, err
	}
	out.Info(ui.Bold(fmt.Sprintf("Plainship v%s", version.Version)))

	// 前置检查: 服务器地址与 Git, 失败直接报错, 不参与守卫打勾.
	if s.Config.SpaceSite.ServerURL.Get() == "" {
		return nil, i18n.Errorf(i18n.CorePublishNoServerURL)
	}
	if !s.GitAvailable || !revision.IsRepo(s) {
		return nil, i18n.Errorf(i18n.CorePublishNeedGit)
	}

	out.Section(i18n.T(i18n.CorePublishGuards))

	// 守卫 1: 当前源码无未提交变更 → ✓ source clean.
	gs := revision.GitStatus(s)
	for _, cat := range revision.Categories {
		if gs.Changes[cat].HasChanges() {
			return nil, i18n.Errorf(i18n.CorePublishRejectDirty, cat)
		}
	}
	out.Success(i18n.T(i18n.CorePublishGuardClean))

	// 守卫 2: `build/` 由当前源码构建 → ✓ build matches sources.
	current := map[revision.Category]string{}
	for _, cat := range revision.Categories {
		h, err := revision.CategoryHash(s, cat)
		if err != nil {
			return nil, err
		}
		current[cat] = h
	}
	bs, err := state.LoadState(s.Root)
	if err != nil {
		return nil, err
	}
	if bs.LastBuildID == "" || bs.BuildNumber == "" || len(bs.CategoryHashes) == 0 {
		return nil, i18n.Errorf(i18n.CorePublishRejectNotBuilt)
	}
	if !fsutil.IsDir(s.BuildDir()) {
		return nil, i18n.Errorf(i18n.CorePublishRejectNoBuildDir)
	}
	for _, cat := range revision.Categories {
		if bs.CategoryHashes[string(cat)] != current[cat] {
			return nil, i18n.Errorf(i18n.CorePublishRejectOutdated)
		}
	}
	out.Success(i18n.T(i18n.CorePublishGuardFresh))

	// 守卫 3: `build/` 必须由生产构建产生, 链接基础路径一致, 防止 dev 产物被发布.
	if bs.BasePath != builder.BasePath(s, false) {
		return nil, i18n.Errorf(i18n.CorePublishRejectOutdated)
	}
	out.Success(i18n.T(i18n.CorePublishGuardProd))

	// 守卫 4: 渲染器版本必须与当前二进制一致, 防止升级后发布旧渲染.
	if bs.RendererVersion != version.RendererVersion() {
		return nil, i18n.Errorf(i18n.CorePublishRejectOutdated)
	}
	out.Success(i18n.T(i18n.CorePublishGuardRenderer))

	// 阶段 1: 查询服务器状态, 网络等待期间显示 spinner.
	fin := out.Spinner(i18n.T(i18n.CorePublishPublishing, bs.BuildNumber))
	token := s.Config.ServerToken()
	if token == "" {
		token = os.Getenv("PLAINSHIP_TOKEN")
	}
	client := sync.New(s.Config.SpaceSite.ServerURL.Get(), s.Config.SpaceSite.ServerSite.Get(), token)
	published, active, err := client.StatusDetail()
	if err != nil {
		fin("")
		return nil, i18n.Errorf(i18n.CorePublishStatusFail, err)
	}
	fin("")

	// 计算差异, 本地执行.
	m, err := manifest.Read(s.Root, bs.LastBuildID)
	if err != nil {
		return nil, i18n.Errorf(i18n.CorePublishManifestFail, err)
	}
	// 服务器无历史版本, 或激活版本与本地上次构建不一致时执行全量同步:
	// 服务器重建 release.
	fullSync := !published || active != bs.LastBuildID
	diff, err := sync.Diff(s.Root, s.BuildDir(), m, fullSync)
	if err != nil {
		return nil, err
	}
	if diff.UploadCount == 0 && diff.DeleteCount == 0 {
		out.Info(i18n.T(i18n.CorePublishNoChange))
		return &PublishResult{}, nil
	}

	// 阶段 2: 上传, 网络等待期间 spinner 带文件数与字节数.
	bytes := 0
	for _, d := range diff.Upload {
		bytes += len(d)
	}
	fin = out.Spinner(i18n.T(i18n.CorePublishSyncing, bs.BuildNumber, diff.UploadCount, formatBytes(bytes)))
	resp, err := client.SyncWithDiff(s.Root, s.BuildDir(), m, fullSync, diff)
	if err != nil {
		fin("")
		return nil, i18n.Errorf(i18n.CorePublishSyncFail, err)
	}
	fin("")

	out.Success(i18n.T(i18n.CorePublishUploaded2, resp.StoredFiles, resp.DeletedFiles))
	out.Success(i18n.T(i18n.CorePublishActivated, bs.BuildNumber, ui.Cyan(s.Config.SpaceSite.ServerURL.Get())))
	return &PublishResult{Response: resp}, nil
}

// formatBytes 把字节数格式化为可读文本, B/KB/MB.
func formatBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
