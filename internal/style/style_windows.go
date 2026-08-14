//go:build windows

package style

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const enableVirtualTerminalProcessing = 0x0004

// enableVT 为控制台句柄启用 ANSI 转义序列处理 (Windows 10+).
// 失败 (非控制台 / 旧版本终端) 时返回 false, 调用方自动降级为无色.
func enableVT(f *os.File) bool {
	h := f.Fd()
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		return false
	}
	r, _, _ = procSetConsoleMode.Call(h, uintptr(mode|enableVirtualTerminalProcessing))
	return r != 0
}
