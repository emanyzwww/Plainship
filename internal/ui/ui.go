// Package ui 提供 Plainship 全部命令的统一输出入口.
//
// 设计, 见 `docs/output-architecture.md`: 事件流 + 渲染器 + 传输层三层分离.
//   - 命令层 `cli`/`servercli` 与业务层 `core`/`builder` 只依赖本包的 UI 接口.
//   - 颜色/对齐/进度/交互等渲染由实现层统一处理, 调用方无法绕过.
//   - 传输目标 stdout/stderr/文件/网络 通过 Options 注入, 默认标准流.
//
// 文件组织, 对应文档 4.7:
//   - `event.go`             事件模型, Level / Event.
//   - `renderer.go`          渲染格式与分派, Format / renderLine.
//   - `renderer_terminal.go` 终端渲染, ANSI 颜色判定.
//   - `renderer_plain.go`    纯文本渲染契约, 无色 / 进度静默 / 交互放行.
//   - `renderer_json.go`     JSON 事件流渲染.
//   - `slog.go`              日志投影, 事件 → slog.
//   - `progress.go`          进度条与旋转指示器.
//   - `interact.go`          交互, Confirm / Prompt.
//   - `suggest.go`           错误建议映射与错误渲染.
//   - `mark.go`              样式标记系统.
//
// 颜色不通过 API 暴露: 调用方用 Success/Warn 等语义方法表达意图, 需要内嵌关键值时
// 使用包级标记函数 Cyan/Bold 等, 由渲染器解析或剥离.
package ui

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/emanyzwww/plainship/internal/format"
)

// Options 是 New 的配置.
type Options struct {
	Out       io.Writer    // Out 是用户输出目标, 供 Info/Success/Detail 等使用, 默认 os.Stdout.
	Err       io.Writer    // Err 是警告与错误目标, 供 Warn/Error 使用, 默认 os.Stderr.
	In        io.Reader    // In 是交互输入源, 默认 os.Stdin.
	Format    Format       // Format 控制渲染格式, 默认 FormatAuto, 终端 → terminal, 否则 plain.
	Timestamp bool         // Timestamp 为 dev/serve 等长驻进程输出行启用 [HH:MM:SS] 时间戳前缀.
	Logger    *slog.Logger // Logger 是日志投影目标, nil 表示不记日志.
}

// UI 是命令层唯一允许使用的输出入口.
//
// 所有方法都是意图表达, 呈现细节由渲染器决定.
//
// `core`/`builder` 接收 UI 参数, 传 nil 表示静默不输出.
type UI interface {
	// Info 输出普通信息, 默认 stdout.
	Info(text string)
	// Success 输出成功提示, 终端下为绿色.
	Success(text string)
	// Warn 输出警告, 到 stderr, 终端下为黄色.
	Warn(text string)
	// Error 输出错误, 到 stderr, 终端下为红色, 并附下一步建议, 由 `suggest.go` 映射.
	Error(err error)
	// Debug 输出诊断信息, 仅日志投影, 不进入用户可见输出.
	Debug(text string)

	// Detail 输出一对键值, 连续调用自动按标签对齐, 按 CJK 显示宽度.
	// value 可嵌入标记 Cyan/Bold 等表达关键值.
	Detail(label, value string)
	// Table 输出对齐表格, headers 为空时省略表头行.
	Table(headers []string, rows [][]string)
	// Section 输出区块标题, 终端下为粗体, 自动前空行.
	Section(title string)
	// Summary 输出一行统计, 各项以两个空格分隔, 支持标记.
	Summary(parts ...string)

	// Progress 创建一个进度条, 非终端静默, 调用方负责输出最终摘要行.
	Progress(total int) *Progress
	// Spinner 启动一个旋转指示器并返回结束回调, done 非空时结束回调输出该文本行; 非终端静默.
	Spinner(text string) func(done string)

	// Confirm 交互确认: 终端下读取 y/yes, 非终端自动放行返回 true.
	Confirm(prompt string) bool
	// Prompt 交互单行输入: 终端下读取一行, 非终端返回 ErrNonInteractive.
	// secret=true 时输入不回显, Windows 与 Unix 均支持.
	Prompt(label string, secret bool) (string, error)

	// Flush 立即按对齐输出挂起的 Detail.
	// 一般无需调用, 后续任意输出会自动冲刷, 用于命令以 Detail 收尾的场景.
	Flush()

	// Writer 返回底层 stdout writer, 逃生口, 供 SSE 直写等场景, 尽量少用.
	Writer() io.Writer
}

// ui 是 UI 的实现.
type ui struct {
	mu     sync.Mutex // mu 保护全部输出操作, dev/serve 多 goroutine 场景.
	out    io.Writer
	err    io.Writer
	in     io.Reader
	format Format
	ts     bool
	logger *slog.Logger

	pending []detail // pending 是挂起的 Detail, 对齐后统一输出.
	wrote   bool     // wrote 是否已输出过内容, 用于 Section 前空行判定.
}

// Discard 是静默 UI, 所有输出被丢弃.
//
// `core`/`builder` 等中间层接收的 UI 参数为 nil 时, 应改用本单例.
var Discard UI = New(Options{Out: io.Discard, Err: io.Discard, In: strings.NewReader("")})

type detail struct{ label, value string }

// New 创建 UI.
func New(opts Options) UI {
	u := &ui{
		out:    opts.Out,
		err:    opts.Err,
		in:     opts.In,
		format: resolveFormat(opts.Out, opts.Format),
		ts:     opts.Timestamp,
		logger: opts.Logger,
	}
	if u.out == nil {
		u.out = os.Stdout
	}
	if u.err == nil {
		u.err = os.Stderr
	}
	if u.in == nil {
		u.in = os.Stdin
	}
	return u
}

// ---------- 基础输出 ----------

func (u *ui) Info(text string)    { u.emit(text, LevelInfo, false) }
func (u *ui) Success(text string) { u.emit(Green(text), LevelSuccess, false) }
func (u *ui) Warn(text string)    { u.emit(Yellow(text), LevelWarn, true) }

// lock 锁定 UI, 供 Progress/Spinner 等跨方法复合操作使用.
func (u *ui) lock()   { u.mu.Lock() }
func (u *ui) unlock() { u.mu.Unlock() }

func (u *ui) Debug(text string) {
	u.logEvent(Event{Level: LevelDebug, Text: text, Time: time.Now()})
}

func (u *ui) Error(err error) {
	if err == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.flushDetails()
	u.logEvent(Event{Level: LevelError, Text: err.Error(), Time: time.Now()})
	if u.format == FormatJSON {
		u.writeJSON(jsonEvent{Time: nowRFC3339(), Level: "error", Type: "error", Text: RenderMarks(err.Error(), false)})
		return
	}
	u.renderErrorTo(u.err, err)
}

// ---------- 结构化展示 ----------

func (u *ui) Detail(label, value string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.format == FormatJSON {
		u.writeJSON(jsonEvent{Time: nowRFC3339(), Level: "info", Type: "detail", Label: RenderMarks(label, false), Value: RenderMarks(value, false)})
		return
	}
	u.pending = append(u.pending, detail{label: label, value: value})
}

// flushDetails 按标签宽度对齐并输出全部挂起的 Detail.
func (u *ui) flushDetails() {
	if len(u.pending) == 0 {
		return
	}
	ds := u.pending
	u.pending = nil
	w := 0
	for _, d := range ds {
		if n := format.DisplayWidth(RenderMarks(d.label, false)); n > w {
			w = n
		}
	}
	for _, d := range ds {
		pad := w - format.DisplayWidth(RenderMarks(d.label, false))
		u.emitLine(d.label+strings.Repeat(" ", pad+2)+d.value, LevelInfo, u.out)
	}
}

func (u *ui) Table(headers []string, rows [][]string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.flushDetails()
	if u.format == FormatJSON {
		u.writeJSON(jsonEvent{Time: nowRFC3339(), Level: "info", Type: "table", Rows: rows})
		return
	}
	cols := len(headers)
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	if cols == 0 {
		return
	}
	widths := make([]int, cols)
	for i := 0; i < cols; i++ {
		if i < len(headers) {
			widths[i] = format.DisplayWidth(RenderMarks(headers[i], false))
		}
		for _, r := range rows {
			if i < len(r) {
				if n := format.DisplayWidth(RenderMarks(r[i], false)); n > widths[i] {
					widths[i] = n
				}
			}
		}
	}
	line := func(cells []string) string {
		var b strings.Builder
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			b.WriteString(cell)
			if i < cols-1 {
				if pad := widths[i] + 2 - format.DisplayWidth(RenderMarks(cell, false)); pad > 0 {
					b.WriteString(strings.Repeat(" ", pad))
				}
			}
		}
		return b.String()
	}
	if len(headers) > 0 {
		u.emitLine(line(headers), LevelInfo, u.out)
	}
	for _, r := range rows {
		u.emitLine(line(r), LevelInfo, u.out)
	}
}

func (u *ui) Section(title string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.flushDetails()
	if u.format == FormatJSON {
		u.writeJSON(jsonEvent{Time: nowRFC3339(), Level: "info", Type: "section", Text: RenderMarks(title, false)})
		return
	}
	if u.wrote {
		u.emitLine("", LevelInfo, u.out)
	}
	u.emitLine(Bold(title), LevelInfo, u.out)
}

func (u *ui) Summary(parts ...string) {
	if u.format == FormatJSON {
		u.mu.Lock()
		defer u.mu.Unlock()
		u.writeJSON(jsonEvent{Time: nowRFC3339(), Level: "info", Type: "summary", Parts: parts})
		return
	}
	// 注意: 文本分支交给 emit, 由 emit 自行加锁; 不能在持有 mu 时调用, Mutex 不可重入.
	u.emit(strings.Join(parts, "  "), LevelInfo, false)
}

// ---------- 内部 ----------

// emit 是公共出口: 先冲刷挂起的 Detail, 再输出一行.
func (u *ui) emit(text string, level Level, toErr bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.wrote = true
	u.flushDetails()
	e := Event{Level: level, Text: text, Time: time.Now()}
	u.logEvent(e)
	w := u.out
	if toErr {
		w = u.err
	}
	u.renderLine(e, w)
}

// emitLine 输出一行, 不触发 Detail flush.
func (u *ui) emitLine(text string, level Level, w io.Writer) {
	u.wrote = true
	e := Event{Level: level, Text: text, Time: time.Now()}
	u.logEvent(e)
	u.renderLine(e, w)
}

// prefix 返回长驻模式的时间戳前缀.
func (u *ui) prefix() string {
	if !u.ts {
		return ""
	}
	return "[" + time.Now().Format("15:04:05") + "]  "
}

// Flush 立即按对齐输出挂起的 Detail.
func (u *ui) Flush() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.flushDetails()
}

// Writer 返回底层 stdout writer.
func (u *ui) Writer() io.Writer { return u.out }
