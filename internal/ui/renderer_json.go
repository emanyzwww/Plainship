package ui

import (
	"encoding/json"
	"fmt"
)

// jsonEvent 是 JSON 渲染器输出的事件结构.
// 每个事件一行, 供脚本/CI 消费; 标记已剥离, 不含任何 ANSI 序列.
type jsonEvent struct {
	Time  string     `json:"time"`
	Level string     `json:"level"`
	Type  string     `json:"type"`
	Text  string     `json:"text,omitempty"`
	Label string     `json:"label,omitempty"`
	Value string     `json:"value,omitempty"`
	Rows  [][]string `json:"rows,omitempty"`
	Parts []string   `json:"parts,omitempty"`
}

// renderJSONLine 把普通消息事件渲染为一行 JSON, 统一写到 stdout, 不区分 stdout/stderr.
func (u *ui) renderJSONLine(e Event) {
	u.writeJSON(jsonEvent{
		Time:  nowRFC3339(),
		Level: levelString(e.Level),
		Type:  "message",
		Text:  RenderMarks(e.Text, false),
	})
}

// writeJSON 序列化并输出一个事件行.
func (u *ui) writeJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	fmt.Fprintln(u.out, string(b))
}
