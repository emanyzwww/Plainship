package ui

// logEvent 把事件投影到 slog, 标记剥离.
//
// 设计: 事件流是唯一真相, 用户输出与日志是它的两个投影, 对应文档 4.5.
func (u *ui) logEvent(e Event) {
	if u.logger == nil {
		return
	}
	text := RenderMarks(e.Text, false)
	switch e.Level {
	case LevelInfo, LevelSuccess:
		u.logger.Info(text)
	case LevelWarn:
		u.logger.Warn(text)
	case LevelError:
		u.logger.Error(text)
	case LevelDebug:
		u.logger.Debug(text)
	}
}
