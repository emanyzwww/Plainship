// Package version 集中管理 Plainship 的版本号.
// 产品版本只在本包定义, 其余模块一律引用本包;
// 升级版本时只需要修改本文件的 Version.
package version

// Version 是 Plainship 产品版本号.
// 发布构建通过 -ldflags "-X github.com/emanyzwww/plainship/internal/version.Version=<tag>" 注入;
var Version = "0.1.5"

// RendererVersion 返回渲染器版本.
// 渲染输出随版本演进, 发版时强制全量重建, 保证发布产物与当前二进制一致.
func RendererVersion() string {
	return Version
}
