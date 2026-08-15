package ui

import (
	"errors"
	"fmt"
	"io"

	"github.com/emanyzwww/plainship/internal/i18n"
)

// suggestions 映射错误键到下一步建议的消息键.
//
// 设计: 错误消息只描述问题, 建议统一由渲染层以黄色提示行输出, 不塞进错误消息.
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
	// 服务器未启动过, 无令牌文件.
	i18n.CliTokenNotFound: i18n.SuggestServeToken,
}

// SuggestFor 返回错误对应的建议消息键, 无匹配时返回空键.
//
// 支持 Wrapf 包装的错误链.
func SuggestFor(err error) i18n.Key {
	var me *i18n.MsgError
	if !errors.As(err, &me) || me == nil {
		return ""
	}
	return suggestions[me.Key]
}

// RenderError 把错误渲染到指定 writer, 红色错误行 + 黄色建议行.
//
// 供非 UI 场景使用, 命令层请用 UI.Error.
func RenderError(out io.Writer, err error) {
	if err == nil {
		return
	}
	u := New(Options{Out: out, Err: out})
	u.(*ui).renderErrorTo(out, err)
}

// renderErrorTo 把错误与建议渲染到指定 writer, 错误为红色, 建议为黄色.
//
// 供 Error 方法与 RenderError 函数复用.
func (u *ui) renderErrorTo(w io.Writer, err error) {
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, u.prefix()+RenderMarks(Red(i18n.T(i18n.CliRootError, err.Error())), u.colored()))
	if key := SuggestFor(err); key != "" {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, u.prefix()+RenderMarks(Yellow(i18n.T(key)), u.colored()))
	}
}
