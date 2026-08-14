// Package core 是 Plainship 的核心编排层.
// 只负责流程编排 (CreateSpace / Build / Publish / Status / Dev),
// Git 语义 (类别划分, 指纹, 提交协议, 编号) 由 internal/revision 提供.
package core

import (
	"fmt"
	"io"
	"os"

	"github.com/emanyzwww/plainship/internal/builder"
	"github.com/emanyzwww/plainship/internal/fsutil"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/manifest"
	"github.com/emanyzwww/plainship/internal/revision"
	"github.com/emanyzwww/plainship/internal/space"
	"github.com/emanyzwww/plainship/internal/state"
	"github.com/emanyzwww/plainship/internal/sync"
	"github.com/emanyzwww/plainship/internal/version"
)

// PublishResult 是一次 publish 的结果.
type PublishResult struct {
	Response *sync.Response
}

// Publish 发布到服务器: 只发布由已提交源码构建出的 build/ 内容.
// 前置守卫:
//  1. 当前源码无未提交变更 (config/theme/docs 均 clean).
//  2. build/ 必须由当前源码构建 (状态中的类别指纹一致).
func Publish(spaceRoot string, out io.Writer) (*PublishResult, error) {
	printl := func(format string, args ...any) {
		if out != nil {
			fmt.Fprintf(out, format+"\n", args...)
		}
	}
	s, err := space.Load(spaceRoot)
	if err != nil {
		return nil, err
	}
	printl("Plainship v%s", version.Version)
	printl("")

	if s.Config.Server.URL == "" {
		return nil, i18n.Errorf(i18n.CorePublishNoServerURL)
	}
	if !s.GitAvailable || !revision.IsRepo(s) {
		return nil, i18n.Errorf(i18n.CorePublishNeedGit)
	}

	// 守卫 1: 当前源码无未提交变更.
	gs := revision.GitStatus(s)
	for _, cat := range revision.Categories {
		if gs.Changes[cat].HasChanges() {
			return nil, i18n.Errorf(i18n.CorePublishRejectDirty, cat)
		}
	}

	// 守卫 2: build/ 由当前源码构建.
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
	// 守卫 3: build/ 必须由生产构建产生 (链接基础路径一致), 防止 dev 产物被发布.
	if bs.BasePath != builder.BasePath(s, false) {
		return nil, i18n.Errorf(i18n.CorePublishRejectOutdated)
	}
	// 守卫 4: 渲染器版本必须与当前二进制一致, 防止升级后发布旧渲染.
	if bs.RendererVersion != version.RendererVersion() {
		return nil, i18n.Errorf(i18n.CorePublishRejectOutdated)
	}

	// 同步.
	printl(i18n.T(i18n.CorePublishPublishing, bs.BuildNumber))
	token := s.Config.Server.Token
	if token == "" {
		token = os.Getenv("PLAINSHIP_TOKEN")
	}
	client := sync.New(s.Config.Server.URL, s.Config.Server.Site, token)
	published, active, err := client.StatusDetail()
	if err != nil {
		return nil, i18n.Errorf(i18n.CorePublishStatusFail, err)
	}
	m, err := manifest.Read(s.Root, bs.LastBuildID)
	if err != nil {
		return nil, i18n.Errorf(i18n.CorePublishManifestFail, err)
	}
	// 服务器无历史版本, 或服务器的激活版本与本地上次构建不一致 (数据丢失/多客户端场景)
	// 时执行全量同步: 服务器重建 release, 避免陈旧文件残留与残缺站点.
	fullSync := !published || active != bs.LastBuildID
	diff, err := sync.Diff(s.Root, s.BuildDir(), m, fullSync)
	if err != nil {
		return nil, err
	}
	if diff.UploadCount == 0 && diff.DeleteCount == 0 {
		printl(i18n.T(i18n.CorePublishNoChange))
		return &PublishResult{}, nil
	}
	if fullSync {
		printl(i18n.T(i18n.CorePublishFullSync, diff.UploadCount))
	} else {
		printl(i18n.T(i18n.CorePublishDiff, diff.UploadCount, diff.DeleteCount))
	}
	resp, err := client.SyncWithDiff(s.Root, s.BuildDir(), m, fullSync, diff)
	if err != nil {
		return nil, i18n.Errorf(i18n.CorePublishSyncFail, err)
	}
	printl(i18n.T(i18n.CorePublishUploaded, resp.StoredFiles))
	printl(i18n.T(i18n.CorePublishDeleted, resp.DeletedFiles))
	printl("")
	printl(i18n.T(i18n.CorePublishOk, bs.BuildNumber))
	return &PublishResult{Response: resp}, nil
}
