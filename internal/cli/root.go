// Package cli 实现 plainship 客户端命令行界面.
//
// 客户端二进制 `cmd/plainship` 的入口; 服务端命令位于 `internal/servercli`;
// 共享的 CLI 框架件, 语言检测 / 错误渲染 / 控制台编码, 位于 `internal/clifx`.
package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/plainship/internal/clifx"
	"github.com/emanyzwww/plainship/internal/format"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/style"
	"github.com/emanyzwww/plainship/internal/ui"
	"github.com/emanyzwww/plainship/internal/version"
)

// 全局输出标志, 由 cobra 绑定, newUI 读取.
var (
	flagJSON    bool // flagJSON 对应 `--json`, JSON 事件流输出, 机器可读.
	flagVerbose bool // flagVerbose 对应 `--verbose`, 输出调试日志到 stderr.
)

// newRootCmd 是 plainship 的根命令.
func newRootCmd() *cobra.Command {
	var lang string
	var noColor bool
	root := &cobra.Command{
		Use:           "plainship",
		Short:         i18n.T(i18n.CliRootShort),
		Long:          rootLong(),
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version.Version,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// `--lang` 优先级高于环境变量 `PLAINSHIP_LANG`.
			if lang != "" {
				i18n.SetLang(i18n.Parse(lang))
			}
			// `--no-color` 优先级高于环境变量 `NO_COLOR`.
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
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "以 JSON 事件流输出 (机器可读)")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "输出调试日志到 stderr")
	root.AddCommand(newNewCmd())
	root.AddCommand(newCreateCmd())
	root.AddCommand(newBuildCmd())
	root.AddCommand(newPublishCmd())
	root.AddCommand(newConnectCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newPreviewCmd())
	root.AddCommand(newDevCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newVersionCmd())
	return root
}

// rootLong 渲染根命令帮助文本.
func rootLong() string {
	return format.NewLine().
		Text(i18n.T(i18n.CliRootTitle)).Br().Br().
		Text(i18n.T(i18n.CliRootTagline)).
		String()
}

// newUI 从 cobra 命令构造输出入口, 注入 stdout/stderr/stdin.
//
// 各命令统一通过它获取 UI; `--json` 切换 JSON 渲染器, `--verbose` 启用日志投影.
func newUI(cmd *cobra.Command) ui.UI {
	opts := ui.Options{
		Out: cmd.OutOrStdout(),
		Err: cmd.ErrOrStderr(),
		In:  cmd.InOrStdin(),
	}
	if flagJSON {
		opts.Format = ui.FormatJSON
	}
	if flagVerbose {
		opts.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	return ui.New(opts)
}

// Execute 执行客户端 CLI, 错误以当前语言输出到 stderr 并设置退出码.
func Execute() {
	// Windows 控制台默认使用 GBK 编码, 需要切换为 UTF-8 以正确显示中文.
	clifx.SetConsoleUTF8()
	// 语言链: `--lang` 预扫描 > `PLAINSHIP_LANG` > 项目配置 > 全局配置 > 默认 en.
	i18n.SetLang(clifx.DetectLang())
	clifx.ApplyLangEarly(os.Args[1:])
	root := newRootCmd()
	// 顶层错误统一走 ui.Error, 红色错误 + 黄色建议.
	u := ui.New(ui.Options{Err: os.Stderr})
	if err := root.Execute(); err != nil {
		u.Error(err)
		os.Exit(1)
	}
}
