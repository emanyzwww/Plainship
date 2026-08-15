package ui

import (
	"github.com/emanyzwww/plainship/internal/style"
)

// colored 报告当前输出是否启用 ANSI 颜色, 仅终端格式.
//
// 判定复用 style.Enabled: 非终端 / `NO_COLOR` / `--no-color` / Windows VT 不可用,
// 任一条件满足即返回 false, 渲染退化为无色文本.
func (u *ui) colored() bool {
	return u.format == FormatTerminal && style.Enabled(u.out)
}
