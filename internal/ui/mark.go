package ui

import "strings"

// 私有标记协议: \x00u[<kind><text>\x00u]
// 用于在字符串中携带样式意图, 如 Detail 值 / Table 单元格 / Summary 项,
// 由渲染器解析为 ANSI 序列; 非终端 / 日志 / JSON 输出自动剥离, 不产生垃圾字符.
const (
	markOpen  = "\x00u["
	markClose = "\x00u]"
)

// 标记种类, 单字节, 保持紧凑.
const (
	markGreen  = 'g'
	markYellow = 'y'
	markCyan   = 'c'
	markBold   = 'b'
	markDim    = 'd'
	markRed    = 'r'
)

func mark(kind byte, s string) string {
	return markOpen + string(kind) + s + markClose
}

// Green 标记成功语义文本, 终端显示绿色, 用于嵌入 Detail 值 / Table 单元格 / Summary 项.
func Green(s string) string { return mark(markGreen, s) }

// Yellow 标记警告语义文本, 终端显示黄色.
func Yellow(s string) string { return mark(markYellow, s) }

// Cyan 标记关键值文本, 终端显示青色, 如 URL / 构建编号 / token.
func Cyan(s string) string { return mark(markCyan, s) }

// Bold 标记标题或强调文本, 终端显示粗体.
func Bold(s string) string { return mark(markBold, s) }

// Dim 标记次要说明文本, 终端显示暗色.
func Dim(s string) string { return mark(markDim, s) }

// Red 标记错误文本, 终端显示红色.
func Red(s string) string { return mark(markRed, s) }

// ContainsMark 报告文本中是否包含样式标记.
func ContainsMark(s string) bool { return strings.Contains(s, markOpen) }

// RenderMarks 把文本中的样式标记按 color 渲染为 ANSI 转义或剥离.
//
// 标记可任意嵌套, 按配对深度解析; 未知与未闭合标记按原文输出.
func RenderMarks(s string, color bool) string {
	if !ContainsMark(s) {
		return s
	}
	var b strings.Builder
	i := 0
	for i < len(s) {
		if strings.HasPrefix(s[i:], markOpen) {
			j := i + len(markOpen)
			kind := byte(0)
			if j < len(s) {
				kind = s[j]
				j++
			}
			// 扫描配对的 markClose, 嵌套计数.
			content, next, closed := scanMarked(s, j)
			if !closed {
				// 未闭合: 原样输出.
				b.WriteString(s[i:])
				break
			}
			i = next
			inner := RenderMarks(content, color)
			if color {
				if code, ok := ansiCode(kind); ok {
					b.WriteString(code)
					b.WriteString(inner)
					b.WriteString("\x1b[0m")
					continue
				}
			}
			b.WriteString(inner)
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// scanMarked 从 i 扫描到配对的 markClose, 嵌套计数, 返回内容与闭合后的位置.
// closed=false 表示未找到配对闭合.
func scanMarked(s string, i int) (content string, next int, closed bool) {
	depth := 1
	k := i
	for k < len(s) {
		switch {
		case strings.HasPrefix(s[k:], markOpen):
			depth++
			k += len(markOpen)
		case strings.HasPrefix(s[k:], markClose):
			depth--
			if depth == 0 {
				return s[i:k], k + len(markClose), true
			}
			k += len(markClose)
		default:
			k++
		}
	}
	return s[i:], len(s), false
}

// ansiCode 返回标记种类对应的 ANSI 起始序列.
func ansiCode(kind byte) (string, bool) {
	switch kind {
	case markGreen:
		return "\x1b[32m", true
	case markYellow:
		return "\x1b[33m", true
	case markCyan:
		return "\x1b[36m", true
	case markBold:
		return "\x1b[1m", true
	case markDim:
		return "\x1b[2m", true
	case markRed:
		return "\x1b[31m", true
	default:
		return "", false
	}
}
