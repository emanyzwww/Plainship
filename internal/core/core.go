// Package core 是 Plainship 的核心编排层.
// 只负责流程编排 (CreateSpace / Build / Publish / Status / Dev),
// Git 语义 (类别划分, 指纹, 提交协议, 编号) 由 internal/revision 提供.
package core

import (
	"time"
)

// 产品版本统一由 internal/version 包定义, 此处不再重复.
// NowTime 返回当前时间格式化字符串.
func NowTime() string {
	return time.Now().Format("2006-01-02 15:04")
}

func orEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
