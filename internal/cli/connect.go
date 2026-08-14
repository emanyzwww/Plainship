// connect.go 实现 plainship connect: 在客户端配置服务器地址与访问令牌.
package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/config"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/style"
	"github.com/emanyzwww/plainship/internal/sync"
)

// newConnectCmd 实现 plainship connect <服务器地址> [--token <令牌>].
// 在 Plainship Space 内运行: 验证令牌后把 server.url 与 server.token 写入 plainship.yaml.
func newConnectCmd() *cobra.Command {
	var tokenFlag string
	cmd := &cobra.Command{
		Use:   i18n.T(i18n.CliConnectUse),
		Short: i18n.T(i18n.CliConnectShort),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			// 必须在 Space 内运行, 因为要写 plainship.yaml.
			root, err := config.FindRoot(".")
			if err != nil {
				return err
			}
			cfg, err := config.Load(root)
			if err != nil {
				return err
			}

			url := sync.NormalizeServerURL(args[0])
			if url == "" {
				return i18n.Errorf(i18n.CliConnectUrlEmpty)
			}

			token := tokenFlag
			if token == "" {
				// 交互式输入: 从服务器启动信息中复制的访问令牌.
				fmt.Fprint(out, i18n.T(i18n.CliConnectPromptToken))
				line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if err != nil {
					return i18n.Errorf(i18n.CliConnectReadFail, err)
				}
				token = strings.TrimSpace(line)
				if token == "" {
					return i18n.Errorf(i18n.CliConnectTokenEmpty)
				}
			}

			// 验证连接与令牌 (GET status; 令牌错误时服务器返回 401).
			client := sync.New(url, cfg.Server.Site, token)
			if _, err := client.Status(); err != nil {
				return i18n.Errorf(i18n.CliConnectVerifyFail, err)
			}

			cfg.Server.URL = url
			cfg.Server.Token = token
			if err := config.Save(root, cfg); err != nil {
				return i18n.Errorf(i18n.CliConnectSaveFail, err)
			}
			st := style.For(out)
			printf(out, "%s\n", st.Green(i18n.T(i18n.CliConnectOk, url)))
			printf(out, "%s\n", i18n.T(i18n.CliConnectNext, cfg.Server.Site))
			return nil
		},
	}
	cmd.Flags().StringVar(&tokenFlag, "token", "", i18n.T(i18n.CliConnectFlagToken))
	return cmd
}
