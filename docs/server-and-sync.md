# 服务器与同步协议

> Plainship Server 只做三件事：**存储 + 同步 + 静态 HTTP**。
> 使用层面（安装、连接、发布）见 [README「部署」](../README.md#部署)。

## 设计约束

- 无数据库、无 Node.js、无构建依赖
- 数据目录在启动时自动创建；站点子目录在收到首次同步后生成
- 服务器永远不做 Markdown 编译 / SSR / 数据库
- 所有复杂计算（Markdown、主题、未来 Vue / React、搜索、SEO）都在客户端完成

## 同步协议（版本化）

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

- `files`：本次要上传的文件（内容为 base64），`deletes`：本次要删除的文件
- `fullSync: true` 时服务器清空该 buildId 的 release 目录后整体重建（不继承旧版本），
  用于服务器无历史版本或激活版本与本地构建不一致的场景，防止陈旧文件残留

## API

```bash
# 状态接口
GET /api/v1/sites/:siteId/status

# 同步接口
POST /api/v1/sites/:siteId/sync
```

## 数据布局与原子发布

- 文件系统存储：`data/sites/<siteId>/releases/<buildId>/`
- 原子发布：上传新构建 → 校验 → 激活 `current` 指针，绝无半发布状态
- 构建版本可回滚

## 认证与安全

- Token 认证（Bearer），**认证永远开启**：服务器不存在「无认证」状态，客户端 `publish` 必须携带令牌
- 令牌保存在 `<数据目录>/server.token`（0600），重启不变；忘记令牌可运行 `plainship token --data <目录>` 查看
- 令牌不写入 `plainship.yaml`（避免随 config 类别提交进 Git 历史），
  通过 `plainship connect` 写入 `.plainship/server.token`，或通过环境变量 `PLAINSHIP_TOKEN` 提供
- 路径遍历防护

## 站点服务

- 访问根路径 `http://localhost:9090/` 时，服务器服务**最近激活**的已发布站点（单站点场景即该站点；没有任何已发布站点时返回"尚未发布"提示）
- 显式指定站点：`http://localhost:9090/?site=<siteId>`
- 客户端修改 `site.siteId` / `server.site` 重新发布后，根路径自动切换到新站点
- 未来：基于 Host 的多站点路由

## 独立部署

`build/` 本身可以脱离 Plainship Server 独立部署（Nginx / GitHub Pages / 任意静态托管），
站点内容与服务器完全解耦。
