// Package protocol 定义 Plainship 同步协议的类型与版本.
// 客户端 (internal/sync) 与服务端 (internal/server) 都依赖本包,
// 协议变更时由 ProtocolVersion 显式协商, 避免两边的 JSON 结构漂移.
package protocol

// ProtocolVersion 是当前同步协议版本.
const ProtocolVersion = 1

// FilePayload 是一个待同步的文件.
type FilePayload struct {
	Path    string `json:"path"`    // 相对站点根的路径, 例如 测试文档/index.html
	Content string `json:"content"` // Base64 编码的文件内容
	Hash    string `json:"hash"`    // 文件内容哈希
}

// Request 是同步请求体.
type Request struct {
	ProtocolVersion int           `json:"protocolVersion"`
	SiteID          string        `json:"siteId"`
	BuildID         string        `json:"buildId"`
	Files           []FilePayload `json:"files"`
	Deletes         []string      `json:"deletes"` // 相对路径列表
	// FullSync 为 true 时服务器清空目标 release 目录后整体重建 (不继承旧版本),
	// 用于服务器无历史版本或与本地构建不一致的场景, 防止陈旧文件残留.
	FullSync bool `json:"fullSync"`
}

// Response 是服务器返回结果.
type Response struct {
	OK           bool   `json:"ok"`
	Message      string `json:"message"`
	BuildID      string `json:"buildId"`
	Active       bool   `json:"active"`
	StoredFiles  int    `json:"storedFiles"`
	DeletedFiles int    `json:"deletedFiles"`
}
