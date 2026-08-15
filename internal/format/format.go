// Package format 提供链式的文本排版构建器.
// 排版职责 (换行/对齐/缩进) 全部由本包承担, 调用方不硬编码空格:
//
//	s := format.NewLine().
//		Text(i18n.T(i18n.CliRootTitle)).Br().Br().
//		Indent(2).Text(usage).Tab().Text(desc).Br().
//		String()
//
// 列对齐按显示宽度计算 (CJK 等宽字符占 2 列), 中英混排不会错位.
//
// 设计: 标准库 `text/tabwriter` 按 rune 数计宽, CJK 算 1 列, 因此自行实现对齐.
package format

import (
	"fmt"
	"strings"
)

// Line 是文本排版构建器, 所有方法返回接收者以支持链式调用.
type Line struct {
	rows [][]string // rows 是已完成的行, 每行是 cell 列表, Tab 分隔.
	cur  []string   // cur 是当前行.
}

// NewLine 创建一个空的排版构建器.
func NewLine() *Line {
	return &Line{}
}

// Text 写入普通文本.
func (l *Line) Text(s string) *Line {
	if len(l.cur) == 0 {
		l.cur = []string{""}
	}
	l.cur[len(l.cur)-1] += s
	return l
}

// Textf 写入格式化文本, 经 fmt.Sprintf.
func (l *Line) Textf(format string, args ...any) *Line {
	return l.Text(fmt.Sprintf(format, args...))
}

// Tab 结束当前单元格, 开始新单元格; 同一列在 String 时自动对齐.
func (l *Line) Tab() *Line {
	if len(l.cur) == 0 {
		l.cur = []string{"", ""}
	} else {
		l.cur = append(l.cur, "")
	}
	return l
}

// Br 结束当前行并换行.
func (l *Line) Br() *Line {
	l.rows = append(l.rows, l.cur)
	l.cur = nil
	return l
}

// Empty 写入一个空行, 等价于 Br.
func (l *Line) Empty() *Line {
	return l.Br()
}

// Indent 写入 n 个空格的缩进.
// 排版缩进由本方法表达, 调用方不直接书写空格字符串.
func (l *Line) Indent(n int) *Line {
	return l.Text(strings.Repeat(" ", n))
}

// String 输出最终排版结果.
// 必须在所有内容写入完毕后调用; 重复调用返回相同结果.
func (l *Line) String() string {
	if len(l.cur) > 0 {
		l.rows = append(l.rows, l.cur)
		l.cur = nil
	}
	// 计算各列最大显示宽度, 仅统计多单元格行, 最后一列不参与填充.
	var widths []int
	for _, row := range l.rows {
		if len(row) < 2 {
			continue
		}
		for i := 0; i < len(row)-1; i++ {
			w := displayWidth(row[i])
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			if w > widths[i] {
				widths[i] = w
			}
		}
	}
	var b strings.Builder
	for _, row := range l.rows {
		for i, cell := range row {
			b.WriteString(cell)
			if i < len(row)-1 && i < len(widths) {
				if pad := widths[i] + 2 - displayWidth(cell); pad > 0 {
					b.WriteString(strings.Repeat(" ", pad))
				}
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// DisplayWidth 估算字符串的显示宽度: ASCII 占 1 列, CJK 等宽字符占 2 列.
// 供排版与对齐场景复用, 如 `internal/ui` 的输出对齐.
func DisplayWidth(s string) int {
	w := 0
	for _, r := range s {
		if isWide(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// displayWidth 是 DisplayWidth 的小写别名, 包内兼容.
func displayWidth(s string) int { return DisplayWidth(s) }

// isWide 判断字符是否为宽字符, 东亚全角/表意字符等.
func isWide(r rune) bool {
	return r >= 0x1100 && (r <= 0x115F || // Hangul Jamo
		r == 0x2329 || r == 0x232A ||
		(r >= 0x2E80 && r <= 0xA4CF && r != 0x303F) || // CJK 等
		(r >= 0xAC00 && r <= 0xD7A3) || // Hangul Syllables
		(r >= 0xF900 && r <= 0xFAFF) || // CJK 兼容
		(r >= 0xFE30 && r <= 0xFE4F) || // CJK 兼容形式
		(r >= 0xFF00 && r <= 0xFF60) || // 全角
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x20000 && r <= 0x2FFFD) ||
		(r >= 0x30000 && r <= 0x3FFFD))
}
