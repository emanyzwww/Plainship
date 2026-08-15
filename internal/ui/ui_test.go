package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// newTest 创建绑定 buffer 的 UI, plain 模式.
func newTest() (*ui, *bytes.Buffer, *bytes.Buffer) {
	var out, errb bytes.Buffer
	return New(Options{Out: &out, Err: &errb, In: strings.NewReader("")}).(*ui), &out, &errb
}

func TestInfoSuccessWarn(t *testing.T) {
	u, out, errb := newTest()
	u.Info("hello")
	u.Success("done")
	u.Warn("careful")
	if out.String() != "hello\ndone\n" {
		t.Errorf("stdout = %q", out.String())
	}
	if errb.String() != "careful\n" {
		t.Errorf("stderr = %q", errb.String())
	}
}

func TestMarksStrippedInPlain(t *testing.T) {
	u, out, _ := newTest()
	u.Info(Cyan("https://x") + " " + Bold("b") + " " + Green("g") + " " + Yellow("y") + " " + Dim("d") + " " + Red("r"))
	if strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "\x00") {
		t.Errorf("plain 输出含转义: %q", out.String())
	}
	if out.String() != "https://x b g y d r\n" {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestRenderMarks(t *testing.T) {
	if got := RenderMarks(Cyan("x"), true); got != "\x1b[36mx\x1b[0m" {
		t.Errorf("RenderMarks(cyan,true) = %q", got)
	}
	if got := RenderMarks(Cyan("x"), false); got != "x" {
		t.Errorf("RenderMarks(cyan,false) = %q", got)
	}
	if got := RenderMarks("plain", true); got != "plain" {
		t.Errorf("RenderMarks(plain) = %q", got)
	}
	if got := RenderMarks(Bold("b")+Cyan("c"), true); got != "\x1b[1mb\x1b[0m\x1b[36mc\x1b[0m" {
		t.Errorf("RenderMarks(nested) = %q", got)
	}
}

func TestRenderMarksNested(t *testing.T) {
	// 嵌套标记: 外层与内层都渲染, 无垃圾残留.
	got := RenderMarks(Green(Cyan("http://x")), true)
	want := "\x1b[32m\x1b[36mhttp://x\x1b[0m\x1b[0m"
	if got != want {
		t.Errorf("嵌套渲染 = %q, want %q", got, want)
	}
	if got := RenderMarks(Green(Cyan("http://x")), false); got != "http://x" {
		t.Errorf("嵌套剥离 = %q", got)
	}
}

func TestDetailAlignment(t *testing.T) {
	u, out, _ := newTest()
	u.Detail("Server URL", Cyan("http://x"))
	u.Detail("Data dir", "./data")
	u.Info("end")
	want := "Server URL  http://x\nData dir    ./data\nend\n"
	if out.String() != want {
		t.Errorf("Detail 对齐错误:\ngot  %q\nwant %q", out.String(), want)
	}
}

func TestDetailCJKAlignment(t *testing.T) {
	u, out, _ := newTest()
	u.Detail("空间", "my-docs")
	u.Detail("站点", "site")
	u.Info("x")
	want := "空间  my-docs\n站点  site\nx\n"
	if out.String() != want {
		t.Errorf("CJK 对齐错误:\ngot  %q\nwant %q", out.String(), want)
	}
}

func TestTable(t *testing.T) {
	u, out, _ := newTest()
	u.Table([]string{"cat", "change"}, [][]string{
		{"config", Green("+1 added")},
		{"theme", Green("clean")},
		{"docs", "+3 added"},
	})
	want := "cat     change\nconfig  +1 added\ntheme   clean\ndocs    +3 added\n"
	if out.String() != want {
		t.Errorf("Table 错误:\ngot  %q\nwant %q", out.String(), want)
	}
}

func TestTableNoHeaders(t *testing.T) {
	u, out, _ := newTest()
	u.Table(nil, [][]string{{"a", "1"}, {"longer", "2"}})
	want := "a       1\nlonger  2\n"
	if out.String() != want {
		t.Errorf("Table 无表头错误: got %q want %q", out.String(), want)
	}
}

func TestSection(t *testing.T) {
	u, out, _ := newTest()
	u.Info("first")
	u.Section("Git")
	u.Section("Build")
	want := "first\n\nGit\n\nBuild\n"
	if out.String() != want {
		t.Errorf("Section 错误: got %q want %q", out.String(), want)
	}
}

func TestSummary(t *testing.T) {
	u, out, _ := newTest()
	u.Summary("8 changed", Green("3 copied"), "1 deleted")
	want := "8 changed  3 copied  1 deleted\n"
	if out.String() != want {
		t.Errorf("Summary 错误: got %q want %q", out.String(), want)
	}
}

func TestErrorWithSuggestion(t *testing.T) {
	u, _, errb := newTest()
	u.Error(errors.New("boom"))
	s := errb.String()
	if !strings.Contains(s, "Error: boom") {
		t.Errorf("Error 缺少消息: %q", s)
	}
}

func TestErrorNil(t *testing.T) {
	u, _, errb := newTest()
	u.Error(nil)
	if errb.Len() != 0 {
		t.Errorf("Error(nil) 不应输出: %q", errb.String())
	}
}

func TestProgressSilentInPlain(t *testing.T) {
	u, out, _ := newTest()
	p := u.Progress(10)
	p.Set(5, "docs/x.md")
	p.Done()
	if out.Len() != 0 {
		t.Errorf("plain 模式 Progress 不应输出: %q", out.String())
	}
}

func TestProgressTerminalDraw(t *testing.T) {
	var out bytes.Buffer
	u := New(Options{Out: &out, Err: &out, In: strings.NewReader(""), Format: FormatTerminal}).(*ui)
	p := u.Progress(2)
	p.Set(1, "")
	p.Done()
	s := out.String()
	if !strings.Contains(s, "1/2") {
		t.Errorf("进度条缺少计数: %q", s)
	}
}

func TestSpinnerSilentInPlain(t *testing.T) {
	u, out, _ := newTest()
	fin := u.Spinner("working")
	fin("")
	if out.Len() != 0 {
		t.Errorf("plain 模式 Spinner 不应输出: %q", out.String())
	}
}

func TestConfirmNonInteractive(t *testing.T) {
	u, _, _ := newTest()
	if !u.Confirm("proceed?") {
		t.Error("非终端 Confirm 应放行返回 true")
	}
}

func TestConfirmParse(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"Y\n", true},
		{"YES\n", true},
		{"\n", false},
		{"n\n", false},
		{"N\n", false},
		{"no\n", false},
		{"maybe\n", false},
	}
	for _, c := range cases {
		if got := confirmParse(c.in); got != c.want {
			t.Errorf("confirmParse(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestPromptNonInteractive(t *testing.T) {
	u, _, _ := newTest()
	if _, err := u.Prompt("token:", true); err != ErrNonInteractive {
		t.Errorf("非终端 Prompt 应返回 ErrNonInteractive, got %v", err)
	}
}

func TestJSONMode(t *testing.T) {
	var out bytes.Buffer
	u := New(Options{Out: &out, Err: &out, In: strings.NewReader(""), Format: FormatJSON}).(*ui)
	u.Info("hello")
	u.Detail("k", "v")
	u.Success("ok")
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("JSON 事件数 = %d, 输出: %q", len(lines), out.String())
	}
	for i, l := range lines {
		var e map[string]any
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			t.Fatalf("第 %d 行不是合法 JSON: %v (%q)", i, err, l)
		}
	}
	if !strings.Contains(lines[0], "\"type\":\"message\"") || !strings.Contains(lines[0], "\"level\":\"info\"") {
		t.Errorf("事件结构错误: %s", lines[0])
	}
	if !strings.Contains(lines[1], "\"type\":\"detail\"") || !strings.Contains(lines[1], "\"label\":\"k\"") {
		t.Errorf("Detail 事件错误: %s", lines[1])
	}
}

func TestTimestampPrefix(t *testing.T) {
	var out bytes.Buffer
	u := New(Options{Out: &out, Err: &out, In: strings.NewReader(""), Timestamp: true}).(*ui)
	u.Info("x")
	if !strings.HasPrefix(out.String(), "[") || !strings.Contains(out.String(), "]  x\n") {
		t.Errorf("时间戳前缀错误: %q", out.String())
	}
}

func TestWriter(t *testing.T) {
	u, out, _ := newTest()
	if u.Writer() != out {
		t.Error("Writer() 应返回 Out")
	}
}

func TestDebugOnlyLogs(t *testing.T) {
	var out bytes.Buffer
	u := New(Options{Out: &out, Err: &out, In: strings.NewReader("")}).(*ui)
	u.Debug("hidden")
	if out.Len() != 0 {
		t.Errorf("Debug 不应进入用户输出: %q", out.String())
	}
}
