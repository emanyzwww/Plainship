package cli

import (
	"io"

	"github.com/emanyzwww/plainship/internal/clifx"
)

// printf / println 是 clifx 输出辅助的薄封装, 保持命令代码简洁.
// 服务端 CLI (internal/servercli) 直接使用 clifx 的公开函数.
func printf(out io.Writer, format string, args ...any) {
	clifx.Printf(out, format, args...)
}

func println(out io.Writer, args ...any) {
	clifx.Println(out, args...)
}
