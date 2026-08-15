//go:build windows

package ui

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32UI           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleModeUI = kernel32UI.NewProc("GetConsoleMode")
	procSetConsoleModeUI = kernel32UI.NewProc("SetConsoleMode")
)

// enableEchoInput 是 Windows 控制台输入模式位 ENABLE_ECHO_INPUT.
const enableEchoInput = 0x0004

// noEcho 启用/禁用控制台回显, Windows 下经 SetConsoleMode.
func noEcho(r io.Reader, on bool) {
	f, ok := r.(*os.File)
	if !ok {
		return
	}
	fd := f.Fd()
	var mode uint32
	r1, _, _ := procGetConsoleModeUI.Call(fd, uintptr(unsafe.Pointer(&mode)))
	if r1 == 0 {
		return
	}
	if on {
		mode &^= enableEchoInput
	} else {
		mode |= enableEchoInput
	}
	procSetConsoleModeUI.Call(fd, uintptr(mode))
}
