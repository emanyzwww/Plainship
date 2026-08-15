//go:build darwin

package ui

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

// noEcho 启用/禁用终端回显, macOS 下经 TIOCGETA/TIOCSETA.
func noEcho(r io.Reader, on bool) {
	f, ok := r.(*os.File)
	if !ok {
		return
	}
	fd := f.Fd()
	var termios syscall.Termios
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TIOCGETA, uintptr(unsafe.Pointer(&termios)), 0, 0, 0); errno != 0 {
		return
	}
	if on {
		termios.Lflag &^= syscall.ECHO
	} else {
		termios.Lflag |= syscall.ECHO
	}
	syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TIOCSETA, uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
}
