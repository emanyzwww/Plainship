// Package proto 定义客户端与服务端之间的部署协议 (跨模块共享).
//
// 传输约定:
//   - 请求: multipart/form-data, 字段 meta (DeployRequest JSON),
//     bundle 为 gzip 压缩的 tar 包体;
//   - 鉴权: Authorization: Bearer <token>,
//     服务端通过 PAPERSHIP_TOKEN 环境变量配置, 未配置则不校验;
//   - 响应: 200 + DeployResponse JSON; 非 200 视为部署失败 (响应体含 message).
package proto

// DeployPath 是部署端点.
const DeployPath = "/api/deploy"

// DeployRequest 是部署请求的元数据 (与 bundle 一起经 multipart 传输).
type DeployRequest struct {
	SiteID      string `json:"site_id"`      // SiteID 站点唯一标识 (同时是服务端目录名).
	PayloadType string `json:"payload_type"` // PayloadType 包体格式, 固定 "tar.gz".
	Files       int    `json:"files"`        // Files 包内文件数.
	Size        int64  `json:"size"`         // Size 压缩包字节数.
}

// DeployResponse 是部署结果.
type DeployResponse struct {
	OK       bool   `json:"ok"`       // OK 是否成功.
	Revision string `json:"revision"` // Revision 本次部署的版本标识.
	SiteURL  string `json:"site_url"` // SiteURL 部署后可访问的地址.
	Message  string `json:"message"`  // Message 人类可读的描述 (失败时为原因).
}
