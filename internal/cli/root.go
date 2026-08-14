// Package cli 实现 plainship 命令行界面.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/emanyzwww/Plainship/internal/cliconfig"
	"github.com/emanyzwww/Plainship/internal/config"
	"github.com/emanyzwww/Plainship/internal/format"
	"github.com/emanyzwww/Plainship/internal/i18n"
	"github.com/emanyzwww/Plainship/internal/style"
	"github.com/emanyzwww/Plainship/internal/version"
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
	root.AddCommand(newNewCmd())
	root.AddCommand(newCreateCmd())
	root.AddCommand(newBuildCmd())
	root.AddCommand(newPublishCmd())
	root.AddCommand(newConnectCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newPreviewCmd())
	root.AddCommand(newServeCmd())
	root.AddCommand(newTokenCmd())
	root.AddCommand(newDevCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newConfigCmd())
	return root
}

// rootLong 渲染根命令帮助文本.
func rootLong() string {
	return format.NewLine().
		Text(i18n.T(i18n.CliRootTitle)).Br().Br().
		Text(i18n.T(i18n.CliRootTagline)).
		String()
}

// applyLangEarly 在构造命令树之前预扫描 --lang 参数.
// cobra 的 Short/Long/flag 描述在命令构造时求值, 因此必须在此之前确定语言,
// 否则 --lang 切换只影响运行期消息, 帮助文本仍保持默认语言.
// 优先级: --lang 参数 > PLAINSHIP_LANG 环境变量 > 默认 (en).
func applyLangEarly(args []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		var val string
		switch {
		case a == "--lang":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				val = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--lang="):
			val = strings.TrimPrefix(a, "--lang=")
		default:
			continue
		}
		if val != "" {
			i18n.SetLang(i18n.Parse(val))
		}
	}
}

// detectLang 返回 CLI 工具语言.
// 优先级: PLAINSHIP_LANG 环境变量 > 项目配置 (lang) > 全局配置 (lang) > 默认 en.
// --lang 参数由 applyLangEarly 在更早阶段覆盖.
func detectLang() i18n.Lang {
	if v := os.Getenv("PLAINSHIP_LANG"); v != "" {
		return i18n.Parse(v)
	}
	if root, err := config.FindRoot("."); err == nil {
		if cfg, err := cliconfig.LoadProject(root); err == nil && cfg.Lang != "" {
			return i18n.Parse(cfg.Lang)
		}
	}
	if cfg, err := cliconfig.LoadGlobal(); err == nil && cfg.Lang != "" {
		return i18n.Parse(cfg.Lang)
	}
	return i18n.DefaultLang()
}

// Execute 执行 CLI, 错误以当前语言输出到 stderr 并设置退出码.
func Execute() {
	// Windows 控制台默认使用 GBK 编码, 需要切换为 UTF-8 以正确显示中文.
	setConsoleUTF8()
	// 语言链: --lang 预扫描 > PLAINSHIP_LANG > 项目配置 > 全局配置 > 默认 en.
	i18n.SetLang(detectLang())
	applyLangEarly(os.Args[1:])
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		renderError(os.Stderr, err)
		os.Exit(1)
	}
}

// renderError 以当前语言输出错误与"下一步建议" (CLI 顶层统一入口).
// 错误红色, 建议黄色; 非终端输出自动无色.
func renderError(out io.Writer, err error) {
	st := style.For(out)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, st.Red(i18n.T(i18n.CliRootError, err.Error())))
	if key := suggestFor(err); key != "" {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, st.Yellow(i18n.T(key)))
	}
}
