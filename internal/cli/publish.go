package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/config"
	"github.com/emanyzwww/plainship/internal/core"
	"github.com/emanyzwww/plainship/internal/fsutil"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/layout"
)

// newPublishCmd 实现 plainship publish.
// 交互终端下弹出发布摘要与确认, `--yes` 跳过; 非交互环境自动放行.
func newPublishCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "publish",
		Short: i18n.T(i18n.CliPublishShort),
		Long:  i18n.T(i18n.CliPublishLong),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			u := newUI(cmd)
			root, err := findSpaceRoot(cmd)
			if err != nil {
				return err
			}
			// 二次确认: 交互终端且未 `--yes` 时弹出摘要, 取消则不发布.
			if !yes && !confirmPublish(cmd, root) {
				return nil
			}
			_, err = core.Publish(root, u)
			return err
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, i18n.T(i18n.CliPublishFlagYes))
	return cmd
}

// confirmPublish 在交互终端弹出发布摘要与确认, 返回 true 表示放行.
//
// 交互判定由 ui.Confirm 统一处理: 仅 stdin 与 stdout 都指向终端时才交互,
// 任一被重定向即放行, 如管道/脚本/CI/测试.
func confirmPublish(cmd *cobra.Command, root string) bool {
	// 发布摘要: 站点 / 构建编号 / 文件数 / 目标服务器.
	rep, _ := core.Status(root)
	c, _, _ := config.Load(root, nil)
	buildNum := rep.BuildNumber
	if buildNum == "" {
		buildNum = i18n.T(i18n.CliPreviewUnbuilt)
	}
	files := 0
	if n, err := fsutil.ListFiles(filepath.Join(root, layout.BuildDir)); err == nil {
		files = len(n)
	}
	prompt := i18n.T(i18n.CliPublishConfirm, c.SpaceSite.ServerSite.Get(), buildNum, files, c.SpaceSite.ServerURL.Get())
	if !newUI(cmd).Confirm(prompt) {
		newUI(cmd).Warn(i18n.T(i18n.CliPublishCancelled))
		return false
	}
	return true
}
