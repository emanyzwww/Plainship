package ui

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/emanyzwww/plainship/internal/style"
)

// ErrNonInteractive 表示在非交互环境请求输入, 如脚本/CI/测试.
//
// 调用方应提示用户改用命令行参数, 如 `--token`.
var ErrNonInteractive = errors.New("ui: 非交互环境, 无法读取输入")

// interactive 报告 stdin 与 stdout 是否都指向终端.
func (u *ui) interactive() bool {
	in, ok1 := u.in.(*os.File)
	out, ok2 := u.out.(*os.File)
	return ok1 && ok2 && style.IsTerminal(in) && style.IsTerminal(out)
}

// confirmParse 解析确认输入: y/yes 返回 true, 其余返回 false, 大小写不敏感.
func confirmParse(line string) bool {
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes"
}

// Confirm 输出提示并读取一行, y/yes 返回 true, 其余含 EOF 返回 false, 大小写不敏感.
//
// 非终端或 JSON 模式直接返回 true, 自动化不被挂起.
func (u *ui) Confirm(prompt string) bool {
	if u.format == FormatJSON || !u.interactive() {
		return true
	}
	fmt.Fprint(u.out, prompt+" ")
	line, err := bufio.NewReader(u.in).ReadString('\n')
	if err != nil {
		return false
	}
	return confirmParse(line)
}

// Prompt 输出提示并读取一行输入, 非终端或 JSON 模式返回 ErrNonInteractive.
//
// secret=true 时终端不回显, Windows 控制台与 Unix 终端均支持, 用于令牌等敏感输入.
func (u *ui) Prompt(label string, secret bool) (string, error) {
	if u.format == FormatJSON || !u.interactive() {
		return "", ErrNonInteractive
	}
	fmt.Fprint(u.out, label+" ")
	if secret {
		noEcho(u.in, true)
		defer noEcho(u.in, false)
	}
	line, err := bufio.NewReader(u.in).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
