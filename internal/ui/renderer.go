package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/emanyzwww/plainship/internal/style"
)

// Format 是输出渲染格式.
type Format int

const (
	FormatAuto     Format = iota // FormatAuto 自动判定, 终端 → terminal, 否则 plain.
	FormatTerminal               // FormatTerminal 终端渲染, ANSI 颜色 + 进度条 + 交互.
	FormatPlain                  // FormatPlain 纯文本, 无色, 进度静默, 交互自动放行.
	FormatJSON                   // FormatJSON 结构化 JSON 事件流, 机器可读, 供脚本/CI 消费.
)

// resolveFormat 自动判定输出格式, FormatAuto 时: 终端 → terminal, 否则 plain.
func resolveFormat(out io.Writer, f Format) Format {
	if f != FormatAuto {
		return f
	}
	if file, ok := out.(*os.File); ok && style.IsTerminal(file) {
		return FormatTerminal
	}
	return FormatPlain
}

// renderLine 按当前格式把事件渲染到目标 writer.
//
// Terminal 与 Plain 共用文本路径, 仅颜色判定不同; JSON 输出结构化事件流.
func (u *ui) renderLine(e Event, w io.Writer) {
	if u.format == FormatJSON {
		u.renderJSONLine(e)
		return
	}
	u.renderTextLine(e, w)
}

// renderTextLine 文本渲染: 时间戳前缀 + 标记渲染, 终端着色, 非终端剥离.
func (u *ui) renderTextLine(e Event, w io.Writer) {
	fmt.Fprintln(w, u.prefix()+RenderMarks(e.Text, u.colored()))
}
