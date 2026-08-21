# core/distribution - 分发层

## 定位

打包构建产物, 推送到服务端并原子部署, 提供静态站点服务.

分发是 **单次事务**: 打包/网络/响应任一环节失败都以 error 明确返回 (构建可容错, 发布必须明确失败).

## 管线位置

`scanner → … → output` 末端独立命令: 由 CLI 按命令触发 (分发 ≠ 构建), **不并入 build.Run**.

## 职责边界

| 问题                              | 归属                    |
|-----------------------------------|-------------------------|
| 页面 / 静态资源 / 附加文件落盘    | core/output             |
| 打包 (gzip+tar) / 推送 / 鉴权     | **core/distribution**   |
| 接收部署包 / 原子切换 / 静态服务  | server (internal/api)   |
| 部署协议 (DeployRequest/Response) | shared/proto (跨端共享) |

## 输入与输出

- 输入: `*space.Space` + `Options` (ServerURL/SiteID/Token/Client) — 缺省逐项取
  `Config.ServerURL` / `Config.SiteID` → `Space.Name()` / `LocalConfig.ServerToken`; `Client` 可注入 (测试用 httptest).
- 输出: `*distribution.Result` — `Revision` / `SiteURL` / `Files` / `Bytes`.
- 流程: 打包 BuildDir (条目按 RelPath 排序, gzip+tar) → multipart POST `/api/deploy`
  (meta=DeployRequest JSON, bundle=包体, Authorization: Bearer token) → 解析 DeployResponse.

## 用法

```go
res, err := distribution.Distribute(ctx, sp, distribution.Options{})
if err != nil { /* 单次事务失败, 明确报错 */ }
fmt.Printf("部署 %s 成功: revision=%s site=%s\n", sp.Name(), res.Revision, res.SiteURL)
```

## 约定与扩展点

1. **服务端未配置** (ServerURL 为空) 直接报错, 不静默.
2. **空构建产物拒绝分发**: BuildDir 无文件时报错, 防止把空站推上线.
3. **包体确定性**: 条目按相对路径排序后入包, 同一构建两次打包字节一致, 便于比对.
4. **部署通道扩展缝**: 当前走 HTTP+令牌; 未来 SSH/rsync 等通道可在本层接入.

## 测试

`go test ./core/distribution/...` : 完整推送 (包体内容/排序/鉴权头/meta), 服务端拒绝, 空构建与未配置地址报错, nil 输入,
Stage 冒烟. 服务端侧测试见 `server/internal/api`.
