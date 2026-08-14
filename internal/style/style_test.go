package style

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestFor_NonTerminal 非终端输出目标 (测试 buffer) 一律无色.
func TestFor_NonTerminal(t *testing.T) {
	defer disabled.Store(false)
	disabled.Store(false)
	var buf bytes.Buffer
	s := For(&buf)
	if got := s.Green("ok"); got != "ok" {
		t.Errorf("非终端 Green = %q, 期望原文", got)
	}
	if got := s.Red("err"); got != "err" {
		t.Errorf("非终端 Red = %q, 期望原文", got)
	}
	if got := s.Bold("t"); got != "t" {
		t.Errorf("非终端 Bold = %q, 期望原文", got)
	}
	if got := For(nil).Cyan("x"); got != "x" {
		t.Errorf("nil 输出 Cyan = %q, 期望原文", got)
	}
}

// TestDisabled 全局禁用后即使终端也无色.
func TestDisabled(t *testing.T) {
	defer disabled.Store(false)
	disabled.Store(true)
	var buf bytes.Buffer
	if got := For(&buf).Green("x"); got != "x" {
		t.Errorf("禁用后 Green = %q, 期望原文", got)
	}
}

// TestNO_COLOR 环境变量禁用颜色.
func TestNO_COLOR(t *testing.T) {
	defer disabled.Store(false)
	disabled.Store(false)
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	if got := For(&buf).Yellow("x"); got != "x" {
		t.Errorf("NO_COLOR 下 Yellow = %q, 期望原文", got)
	}
}

// TestIsTerminal_NonTerminal 普通文件与 nil 不是终端.
func TestIsTerminal_NonTerminal(t *testing.T) {
	if IsTerminal(nil) {
		t.Error("nil 应返回 false")
	}
	f, err := os.CreateTemp(t.TempDir(), "style-test")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if IsTerminal(f) {
		t.Error("普通文件应返回 false")
	}
}

// TestWrap 启用状态下包裹 ANSI 序列 (直接构造 S, 绕过终端检测).
func TestWrap(t *testing.T) {
	s := &S{on: true}
	if got := s.Green("ok"); got != codeGreen+"ok"+codeReset {
		t.Errorf("Green = %q", got)
	}
	if got := s.Red("err"); got != codeRed+"err"+codeReset {
		t.Errorf("Red = %q", got)
	}
	if got := s.Cyan("ps-0001"); !strings.HasPrefix(got, codeCyan) || !strings.HasSuffix(got, codeReset) {
		t.Errorf("Cyan = %q", got)
	}
	// 组合样式 (嵌套) 输出多个序列但以 reset 收尾.
	if got := s.Bold(s.Green("done")); !strings.HasSuffix(got, codeReset) {
		t.Errorf("组合样式 = %q", got)
	}
}
