//go:build !linux && !darwin && !windows

package ui

import "io"

// noEcho 回显控制仅支持 linux / darwin / windows.
//
// 其他平台, 如 freebsd, 为空实现: 敏感输入仍会回显, 但不影响功能.
func noEcho(r io.Reader, on bool) {}
