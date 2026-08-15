package ui

import "time"

// Level 是输出事件的语义级别.
type Level int

const (
	LevelInfo    Level = iota // LevelInfo 普通信息级别.
	LevelSuccess              // LevelSuccess 成功级别.
	LevelWarn                 // LevelWarn 警告级别.
	LevelError                // LevelError 错误级别.
	LevelDebug                // LevelDebug 调试级别.
)

// Event 是一次输出的结构化事件, 对应文档 4.2.
//
// 由 UI 方法产生, 同时投影到渲染器与日志, 用户输出和 slog 各取所需.
type Event struct {
	Level Level     // Level 是语义级别.
	Text  string    // Text 是已通过 i18n 渲染的文案, 标记由渲染器解析.
	Time  time.Time // Time 是事件产生时间.
}

// levelString 返回级别的 JSON 字符串表示.
func levelString(l Level) string {
	switch l {
	case LevelSuccess:
		return "success"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelDebug:
		return "debug"
	default:
		return "info"
	}
}

// nowRFC3339 返回当前时间的 RFC3339 文本, 用于 JSON 事件时间戳.
func nowRFC3339() string { return time.Now().Format(time.RFC3339) }
