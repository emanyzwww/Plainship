// connect.go 实现 plainship connect: 在客户端配置服务器地址与访问令牌.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/config"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/sync"
	"github.com/emanyzwww/plainship/internal/ui"
)

// newConnectCmd 实现 plainship connect <服务器地址> [--token <令牌>].
// 在 Plainship Space 内运行: 验证令牌后把 `server.url` 写入 `plainship.yaml`,
// `server.token` 写入空间级客户端配置 `.plainship/config.yaml` (SaveProject 域).
func newConnectCmd() *cobra.Command {
	var tokenFlag string
	cmd := &cobra.Command{
		Use:   i18n.T(i18n.CliConnectUse),
		Short: i18n.T(i18n.CliConnectShort),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := newUI(cmd)
			// 必须在 Space 内运行, 需要写 plainship.yaml.
			root, err := config.FindRoot(".")
			if err != nil {
				return err
			}
			c, _, err := config.Load(root, nil)
			if err != nil {
				return err
			}
			c.SetSpaceRoot(root)

			url := sync.NormalizeServerURL(args[0])
			if url == "" {
				return i18n.Errorf(i18n.CliConnectUrlEmpty)
			}

			token := tokenFlag
			if token == "" {
				// 交互式输入: 从服务器启动信息中复制的访问令牌, secret 模式不回显;
				// 非交互环境返回错误, 提示使用 `--token`.
				tok, err := u.Prompt(i18n.T(i18n.CliConnectPromptToken), true)
				if err != nil {
					return i18n.Errorf(i18n.CliConnectReadFail, err)
				}
				token = tok
				if token == "" {
					return i18n.Errorf(i18n.CliConnectTokenEmpty)
				}
			}

			// 验证连接与令牌, GET status, 令牌错误时服务器返回 401.
			client := sync.New(url, c.SpaceSite.ServerSite.Get(), token)
			if _, err := client.Status(); err != nil {
				return i18n.Errorf(i18n.CliConnectVerifyFail, err)
			}

			if err := c.SpaceSite.ServerURL.Set(url); err != nil {
				return err
			}
			if err := c.SpaceClient.ServerToken.Set(token); err != nil {
				return err
			}
			if _, err := config.Save(c, config.SaveSpace); err != nil {
				return i18n.Errorf(i18n.CliConnectSaveFail, err)
			}
			// 令牌单独持久化到空间级客户端配置 `.plainship/config.yaml`.
			if _, err := config.Save(c, config.SaveProject); err != nil {
				return i18n.Errorf(i18n.CliConnectSaveFail, err)
			}
			u.Info(ui.Green(i18n.T(i18n.CliConnectOk, url)))
			u.Info(i18n.T(i18n.CliConnectNext, c.SpaceSite.ServerSite.Get()))
			return nil
		},
	}
	cmd.Flags().StringVar(&tokenFlag, "token", "", i18n.T(i18n.CliConnectFlagToken))
	return cmd
}
