// Package embed 使用 go:embed 内嵌默认主题, 保证即使 themes 目录缺失也能构建.
package embed

import "embed"

// FS 内嵌的默认主题文件.
// 目录结构: default/theme.json, default/layouts/, default/assets/.
//
//go:embed default
var FS embed.FS
