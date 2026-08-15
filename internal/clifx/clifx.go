// Package clifx 提供客户端与服务端 CLI 共享的框架件:
// 控制台编码, 输出辅助, 语言检测, 错误渲染与建议映射.
// 客户端二进制 (cmd/plainship) 与服务端二进制 (cmd/plainship-server)
// 都依赖本包, 保证两个入口的语言与输出行为一致.
package clifx

import (
	"fmt"
	"io"
	"strings"

	"github.com/emanyzwww/plainship/internal/config"
	"github.com/emanyzwww/plainship/internal/i18n"
	"github.com/emanyzwww/plainship/internal/style"
)

// Printf 向指定输出写入格式化内容.
func Printf(out io.Writer, format string, args ...any) {
	fmt.Fprintf(out, format, args...)
}

// Println 向指定输出写入一行内容.
func Println(out io.Writer, args ...any) {
	fmt.Fprintln(out, args...)
}

// DetectLang 返回 CLI 工具语言.
// 优先级由 cfg 包统一处理: 环境变量 > 项目配置 (lang) > 全局配置 (lang) > 默认 en.
// --lang 参数由 ApplyLangEarly 在更早阶段覆盖.
func DetectLang() i18n.Lang {
	root := ""
	if r, err := config.FindRoot("."); err == nil {
		root = r
	}
	c, _, err := config.Load(root, nil)
	if err != nil {
		return i18n.DefaultLang()
	}
	return i18n.Parse(c.Lang())
}

// ApplyLangEarly 在构造命令树之前预扫描 --lang 参数.
// cobra 的 Short/Long/flag 描述在命令构造时求值, 因此必须在此之前确定语言,
// 否则 --lang 切换只影响运行期消息, 帮助文本仍保持默认语言.
// 优先级: --lang 参数 > PLAINSHIP_LANG 环境变量 > 默认 (en).
func ApplyLangEarly(args []string) {
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

// RenderError 以当前语言输出错误与"下一步建议" (CLI 顶层统一入口).
// 错误红色, 建议黄色; 非终端输出自动无色.
func RenderError(out io.Writer, err error) {
	st := style.For(out)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, st.Red(i18n.T(i18n.CliRootError, err.Error())))
	if key := SuggestFor(err); key != "" {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, st.Yellow(i18n.T(key)))
	}
}
