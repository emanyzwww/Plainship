// Package clifx 提供客户端与服务端 CLI 共享的框架件:
// 控制台编码与工具语言检测.
//
// 客户端二进制 `cmd/plainship` 与服务端二进制 `cmd/plainship-server`
// 都依赖本包, 保证两个入口的语言与输出行为一致.
//
// 输出已统一收敛到 `internal/ui`, UI 接口 + 渲染器, 本包不再承担输出职责.
package clifx

import (
	"strings"

	"github.com/emanyzwww/plainship/internal/config"
	"github.com/emanyzwww/plainship/internal/i18n"
)

// DetectLang 返回 CLI 工具语言.
//
// 优先级由 config 包统一处理: 环境变量 > 项目配置 `lang` > 全局配置 `lang` > 默认 en.
// `--lang` 参数由 ApplyLangEarly 在更早阶段覆盖.
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

// ApplyLangEarly 在构造命令树之前预扫描 `--lang` 参数.
//
// cobra 的 Short/Long/flag 描述在命令构造时求值, 语言必须在构造前确定;
// 优先级: `--lang` 参数 > `PLAINSHIP_LANG` 环境变量 > 默认 en.
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
