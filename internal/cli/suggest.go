package cli

import (
	"errors"

	"github.com/emanyzwww/plainship/internal/i18n"
)

// suggestions 映射错误键到"下一步建议"消息键.
// 错误消息只描述问题, 建议统一由 CLI 层以提示行输出 (黄色),
// 避免把建议文本塞进错误消息 (风格不一致且难以着色).
var suggestions = map[i18n.Key]i18n.Key{
	// 不在 Space 目录内运行.
	i18n.ConfigNotFound: i18n.SuggestCreateSpace,
	i18n.SpaceNotFound:  i18n.SuggestCreateSpace,
	// 缺少 Git.
	i18n.GitNotFound:        i18n.SuggestInstallGit,
	i18n.CoreBuildNeedGit:   i18n.SuggestInstallGit,
	i18n.CorePublishNeedGit: i18n.SuggestInstallGit,
	// 未配置服务器.
	i18n.CorePublishNoServerURL: i18n.SuggestConnectServer,
	i18n.SyncNoServerURL:        i18n.SuggestConnectServer,
	i18n.SyncNoServerURLSync:    i18n.SuggestConnectServer,
	// 发布前需要先构建.
	i18n.CorePublishRejectDirty:      i18n.SuggestBuildFirst,
	i18n.CorePublishRejectNotBuilt:   i18n.SuggestBuildFirst,
	i18n.CorePublishRejectNoBuildDir: i18n.SuggestBuildFirst,
	i18n.CorePublishRejectOutdated:   i18n.SuggestBuildFirst,
	i18n.CliPreviewNotBuilt:          i18n.SuggestBuildFirst,
	// 连接 / 令牌问题.
	i18n.CliConnectVerifyFail: i18n.SuggestCheckServer,
	i18n.SyncConnFail:         i18n.SuggestCheckServer,
	// 服务器未启动过 (无令牌文件).
	i18n.CliTokenNotFound: i18n.SuggestServeToken,
}

// suggestFor 返回错误对应的建议消息键; 无匹配时返回空键.
// 支持被 Wrapf 包装的错误链 (errors.As).
func suggestFor(err error) i18n.Key {
	var me *i18n.MsgError
	if !errors.As(err, &me) || me == nil {
		return ""
	}
	return suggestions[me.Key]
}
