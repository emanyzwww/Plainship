//go:build windows

package cli

import "syscall"

// setConsoleUTF8 在 Windows 上设置控制台代码页为 UTF-8, 正确显示中文.
func setConsoleUTF8() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	kernel32.NewProc("SetConsoleOutputCP").Call(65001)
	kernel32.NewProc("SetConsoleCP").Call(65001)
}
