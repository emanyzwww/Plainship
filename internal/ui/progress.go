package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Progress 是一个从左到右的进度条.
//
// 终端模式: 单行重绘, 条 + 计数 + 当前项; 非终端/JSON: 静默, 调用方负责输出最终摘要行.
type Progress struct {
	u       *ui
	total   int
	n       int
	label   string
	done    bool
	lastLen int
}

// Progress 创建一个进度条, total 为总量, 调用方以 Set 推进.
func (u *ui) Progress(total int) *Progress {
	p := &Progress{u: u, total: total}
	if u.format == FormatTerminal {
		p.draw()
	}
	return p
}

// draw 重绘进度行, 仅终端模式, 非终端/JSON 静默.
func (p *Progress) draw() {
	if p.done || p.u.format != FormatTerminal {
		return
	}
	const width = 20
	frac := 0.0
	if p.total > 0 {
		frac = float64(p.n) / float64(p.total)
	}
	filled := int(frac * float64(width))
	bar := strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", width-filled)
	line := fmt.Sprintf("\r%s %d/%d", bar, p.n, p.total)
	if p.label != "" {
		line += "  " + p.label
	}
	p.lastLen = len(line)
	fmt.Fprint(p.u.out, line)
}

// Set 设置当前进度与当前项文本, 进度范围 0..total, 如正在渲染的文档名.
// label 为空时不改变当前项.
func (p *Progress) Set(n int, label string) {
	if p.n == n && p.label == label {
		return
	}
	p.n = n
	if label != "" {
		p.label = label
	}
	p.draw()
}

// Done 结束进度条并清除进度行.
func (p *Progress) Done() {
	if p.done {
		return
	}
	p.done = true
	if p.u.format == FormatTerminal {
		fmt.Fprint(p.u.out, "\r"+strings.Repeat(" ", p.lastLen)+"\r")
	}
}

// spinnerFrames 是旋转指示器的帧序列.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner 启动一个旋转指示器并返回结束回调, done 非空时回调输出该文本行.
//
// 终端模式: 每 100ms 换帧单行重绘; 非终端/JSON: 不输出, 仅回调解读 done.
func (u *ui) Spinner(text string) func(done string) {
	if u.format != FormatTerminal {
		return func(done string) {
			if done != "" {
				u.Info(done)
			}
		}
	}
	stop := make(chan struct{})
	var mu sync.Mutex
	var stopped bool
	var lastLen int
	draw := func(frame string) {
		mu.Lock()
		defer mu.Unlock()
		if stopped {
			return
		}
		line := "\r" + frame + " " + text
		lastLen = len(line)
		fmt.Fprint(u.out, line)
	}
	draw(spinnerFrames[0])
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		i := 1
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				draw(spinnerFrames[i%len(spinnerFrames)])
				i++
			}
		}
	}()
	var once sync.Once
	return func(done string) {
		once.Do(func() {
			mu.Lock()
			stopped = true
			close(stop)
			fmt.Fprint(u.out, "\r"+strings.Repeat(" ", lastLen)+"\r")
			mu.Unlock()
		})
		if done != "" {
			u.Info(done)
		}
	}
}
