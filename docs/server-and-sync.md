# 服务器部署与同步协议

> Plainship Server 只做三件事:**存储 + 同步 + 静态 HTTP**.
> 客户端使用(创建 / 构建 / 发布)见 [使用指南](usage.md).

## 安装

### 一键安装(推荐)

在服务器上执行一条命令,脚本自动探测平台,下载最新版 `plainship-server`,校验 SHA-256,安装,生成访问令牌并启动服务:

```bash
# Linux / macOS: 安装并启动服务器
curl -fsSL https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.sh | bash

# Windows (PowerShell): 先下载安装脚本, 再运行
Invoke-WebRequest -UseBasicParsing https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.ps1 -OutFile install.ps1
.\install.ps1
```

安装脚本支持固定版本与自定义参数(可复现安装,避免版本漂移):

```bash
# 安装指定版本(推荐生产环境固定版本)
curl -fsSL https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.sh | bash -s -- --version <release-tag>

# 自定义监听地址与数据目录
curl -fsSL https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.sh | bash -s -- --addr :9090 --data /opt/plainship/data
```

> `--no-verify` 会跳过 SHA-256 校验(不推荐);脚本默认在任何一步失败时中止,不会留下残缺安装.

### 手动启动

```bash
plainship-server serve --addr :9090 --data ./data
# 不带 --token 时自动生成令牌并保存到 data/server.token(重启不变)
# 运行日志:--log-level debug|info|warn|error(默认 info)/ --log-file <路径>(默认 stderr)/ --log-format text|json
# 忘记令牌?运行:plainship-server token --data ./data
```

> 认证永远开启:服务器不存在"无认证"状态,客户端 `publish` 必须携带令牌.

## 客户端连接与发布

```bash
# 在 Space 目录配置并验证服务器连接(粘贴服务器打印的访问令牌)
plainship connect http://<服务器地址>:9090

# 发布(只发布由已提交源码构建出的内容)
plainship publish
```

> 令牌写入 `.plainship/config.yaml`(0600,不进 Git),也可用环境变量 `PLAINSHIP_TOKEN` 提供.

## 设计约束

- 无数据库,无 Node.js,无构建依赖
- 数据目录在启动时自动创建;站点子目录在收到首次同步后生成
- 服务器永远不做 Markdown 编译 / SSR / 数据库
- 所有复杂计算(Markdown,主题,SEO)都在客户端完成

## 同步协议(版本化)

```json
{
  "protocolVersion": 1,
  "siteId": "my-docs",
  "buildId": "build-xxx",
  "files": [
    {
      "path": "index.html",
      "content": "<base64>",
      "hash": "..."
    }
  ],
  "deletes": ["old/index.html"],
  "fullSync": false
}
```

- `files`:本次要上传的文件(内容为 base64),`deletes`:本次要删除的文件
- `fullSync: true` 时服务器清空该 buildId 的 release 目录后整体重建(不继承旧版本),
  用于服务器无历史版本或激活版本与本地构建不一致的场景,防止陈旧文件残留

## API

```bash
# 状态接口
GET /api/v1/sites/:siteId/status

# 同步接口
POST /api/v1/sites/:siteId/sync

# 构建元数据接口(需 Bearer Token,返回该 build 的 release.json)
GET /api/v1/sites/:siteId/releases/:buildId
```

## 数据布局与原子发布

- 文件系统存储:`data/sites/<siteId>/releases/<buildId>/`
- 原子发布:上传新构建 → 校验 → 激活 `current` 指针,绝无半发布状态
- 构建版本可回滚

## 认证与安全

- Token 认证(Bearer),**认证永远开启**:服务器不存在"无认证"状态,客户端 `publish` 必须携带令牌
- 令牌保存在 `<数据目录>/server.token`(0600),重启不变;忘记令牌可运行 `plainship-server token --data <目录>` 查看
- 令牌不写入 `plainship.yaml`(避免随 config 类别提交进 Git 历史),
  通过 `plainship connect` 写入 `.plainship/config.yaml`,或通过环境变量 `PLAINSHIP_TOKEN` 提供
- 路径遍历防护

## 站点服务

- 访问根路径 `http://localhost:9090/` 时,服务器服务**最近激活**的已发布站点(单站点场景即该站点;没有任何已发布站点时返回"尚未发布"提示)
- 显式指定站点:`http://localhost:9090/?site=<siteId>`
- 客户端修改 `site.siteId` / `server.site` 重新发布后,根路径自动切换到新站点
- 未来:基于 Host 的多站点路由

## 静态托管(脱离服务器)

`build/` 本身可以脱离 Plainship Server 独立部署(Nginx / GitHub Pages / 任意静态托管),站点内容与服务器完全解耦.