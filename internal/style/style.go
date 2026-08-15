// Package style 提供终端 ANSI 颜色渲染, 用于 CLI 输出的视觉分层.
//
// 自动降级规则, 任何一条满足即返回原文, 不产生转义序列:
//   - 输出目标不是终端, 管道 / 文件 / 测试 buffer.
//   - 环境变量 `NO_COLOR` 已设置, 见 https://no-color.org.
//   - `--no-color` 显式禁用.
//
// 颜色只做渲染期包裹, 不进入 i18n 消息文本.
package style

import (
	"io"
	"os"
	"sync/atomic"
)

// ANSI SGR 码.
const (
	codeReset  = "[0m"
	codeBold   = "[1m"
	codeDim    = "[2m"
	codeRed    = "[31m"
	codeGreen  = "[32m"
	codeYellow = "[33m"
	codeCyan   = "[36m"
)

// disabled 是全局样式开关, 对应 `--no-color`.
var disabled atomic.Bool

// Disable 全局禁用颜色, 等价于 `NO_COLOR` 环境变量.
func Disable() {
	disabled.Store(true)
}

// S 是绑定输出目标的样式渲染器.
type S struct {
	on bool // on 是否启用颜色.
}

// Enabled 报告输出目标是否启用 ANSI 颜色.
// 判定规则与 For 一致, 终端 + 非 `NO_COLOR` + 非 `--no-color` + VT 可用.
func Enabled(w io.Writer) bool { return enabled(w) }

// For 为输出目标创建渲染器.
// 目标不是终端时自动无色, 所有样式方法原样返回文本.
func For(w io.Writer) *S {
	if w == nil {
		return &S{}
	}
	return &S{on: enabled(w)}
}

// enabled 判断输出目标是否支持 ANSI 颜色.
func enabled(w io.Writer) bool {
	if disabled.Load() || os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok || !IsTerminal(f) {
		return false
	}
	// Windows 控制台需要显式启用 VT 处理, Unix 终端默认支持.
	return enableVT(f)
}

// IsTerminal 报告 f 是否指向终端, 字符设备.
// 用于交互功能, 如 publish 确认; 管道 / 文件 / 测试注入的 reader 均非终端.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// wrap 应用样式; 未启用时原样返回.
func (s *S) wrap(code, text string) string {
	if !s.on {
		return text
	}
	return code + text + codeReset
}

// Green 绿色, 表示成功.
func (s *S) Green(text string) string { return s.wrap(codeGreen, text) }

// Red 红色, 表示错误.
func (s *S) Red(text string) string { return s.wrap(codeRed, text) }

// Yellow 黄色, 表示警告 / 需要注意.
func (s *S) Yellow(text string) string { return s.wrap(codeYellow, text) }

// Cyan 青色, 表示编号 / URL 等关键值.
func (s *S) Cyan(text string) string { return s.wrap(codeCyan, text) }

// Bold 粗体, 表示标题.
func (s *S) Bold(text string) string { return s.wrap(codeBold, text) }

// Dim 暗色, 表示次要信息.
func (s *S) Dim(text string) string { return s.wrap(codeDim, text) }
