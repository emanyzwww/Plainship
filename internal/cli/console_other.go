//go:build !windows

package cli

// setConsoleUTF8 在非 Windows 平台无需处理.
func setConsoleUTF8() {}
