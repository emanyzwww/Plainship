//go:build !windows

package style

import "os"

// enableVT Unix 终端默认支持 ANSI 转义序列, 无需额外配置.
func enableVT(_ *os.File) bool { return true }
