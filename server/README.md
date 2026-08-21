# server - PaperShip 服务端

接收客户端部署的站点包, 原子切换到站点目录, 提供静态站点服务.

## 运行

```sh
go run ./cmd/server            # 默认 :8080, 站点目录 ./sites
PORT=9090 PAPERSHIP_SITES=/var/sites PAPERSHIP_TOKEN=secret go run ./cmd/server
```

- `PORT`: 监听端口, 默认 8080.
- `PAPERSHIP_SITES`: 站点存放目录, 默认 `./sites`.
- `PAPERSHIP_TOKEN`: 部署令牌; 设置后客户端必须带 `Authorization: Bearer <token>`, 未设置则不校验.

## 端点

- `POST /api/deploy`: multipart (meta=DeployRequest JSON + bundle=gzip+tar), 解包到临时目录后 原子切换 (旧目录先移走,
  失败回滚), 部署协议见 `shared/proto`.
- `GET /<siteID>/...`: 静态站点服务 (站点根 `/demo/` 出 `index.html`).

## 安全边界

- 站点标识 (`SiteID`) 拒绝路径分隔符与 `..`;
- 包内路径拒绝越界 (`../` / 绝对路径 / 盘符) 与符号链接, 防目录穿越;
- 包体上限 512MiB.

## 测试

`go test ./...` : 完整部署+静态访问, 站点标识与包内路径越界拒绝, 令牌鉴权.
