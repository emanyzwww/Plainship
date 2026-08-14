// Package servercli 实现 plainship-server 命令行界面.
// 服务端二进制 (cmd/plainship-server) 的入口, 与客户端 CLI (internal/cli) 分离:
// 只包含 serve / token / version 三个命令, 不含任何客户端构建与发布逻辑.
// 共享的 CLI 框架件 (语言检测 / 错误渲染 / 控制台编码) 位于 internal/clifx.
package servercli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/clifx"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/style"
	"github.com/emanyzwww/plainship/internal/version"
)

// newRootCmd 是 plainship-server 的根命令.
func newRootCmd() *cobra.Command {
	var lang string
	var noColor bool
	root := &cobra.Command{
		Use:           "plainship-server",
		Short:         i18n.T(i18n.CliRootShort),
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version.Version,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// --lang 优先级高于环境变量 PLAINSHIP_LANG.
			if lang != "" {
				i18n.SetLang(i18n.Parse(lang))
			}
			// --no-color 优先级高于环境变量 NO_COLOR.
			if noColor {
				style.Disable()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.PersistentFlags().StringVar(&lang, "lang", "", i18n.T(i18n.CliFlagLang))
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, i18n.T(i18n.CliFlagNoColor))
	root.AddCommand(newServeCmd())
	root.AddCommand(newTokenCmd())
	root.AddCommand(newVersionCmd())
	return root
}

// Execute 执行服务端 CLI, 错误以当前语言输出到 stderr 并设置退出码.
func Execute() {
	// Windows 控制台默认使用 GBK 编码, 需要切换为 UTF-8 以正确显示中文.
	clifx.SetConsoleUTF8()
	// 语言链: --lang 预扫描 > PLAINSHIP_LANG > 项目配置 > 全局配置 > 默认 en.
	i18n.SetLang(clifx.DetectLang())
	clifx.ApplyLangEarly(os.Args[1:])
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		clifx.RenderError(os.Stderr, err)
		os.Exit(1)
	}
}
