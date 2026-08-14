//go:build !windows

package clifx

// SetConsoleUTF8 在非 Windows 平台无需处理.
func SetConsoleUTF8() {}
