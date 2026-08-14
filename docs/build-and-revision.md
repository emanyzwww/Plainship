# 构建、提交与编号

> 本文介绍 `plainship build` 的完整机制：变更检测、分步提交、构建编号与历史回滚。
> 使用层面见 [README「快速开始」](../README.md#quick-start)。

## 构建流程

`plainship build` 的完整流程：

1. 检测变更：`config` / `themes` / `docs` 三类各自统计
2. 增量构建到 `build/`（失败则直接退出，不产生任何提交与编号）
3. 成功后分步提交，跳过无变更的类别：
   - `config`：`plainship.yaml` + `.gitignore`
   - `theme`：`themes/`
   - `docs`：`docs/`
4. 打构建编号 tag `ps-0001`、`ps-0002`、……指向最后一个提交

## 机器提交协议

提交信息由程序生成，采用统一机器格式，可供程序解析与校验：

```
<类别> build=<编号> hash=<内容指纹16位>
```

例如：

```
f0e1a2b (tag: ps-0003)  docs build=ps-0003 hash=3f4a9c2d1b8e7a0f
c8d9e0f                 theme build=ps-0003 hash=a1c2e3f4059670d8
a1b2c3d                 config build=ps-0003 hash=9d8e7f6a5b4c3d2e
```

- `build=<编号>` 把一次构建的三步提交串起来
- `hash=<指纹>` 是该类别内容的 SHA-256 联合指纹，用于校验「当前源码 == 已提交内容」
- `-m "消息"` 只作为 docs 提交的备注写入 body，不影响机器解析
- 渲染器 / 二进制版本升级后，即使源码无变更 `plainship build` 也会重建（不再早退），`plainship publish` 也会拒绝发布旧版本渲染的产物，避免旧 HTML 上线

## 历史回滚

每次成功构建都有一个编号，编号记录在 Git tag 中，天然保留历史：

- **单篇回滚**：`git log -- docs/某篇.md` 查看该篇历史 → `git checkout ps-0003 -- docs/某篇.md` 还原旧版 → `plainship build`
- **整站回滚**：`git checkout ps-0003` → `plainship build` → `plainship publish`
- **撤销某类改动**：例如只想撤销主题改动，`git revert` 对应的 theme 提交即可

回滚本质是「还原源码 + 重新构建」：构建输入是 docs + themes + config，加上 Plainship 版本，因此构建可复现。

## 链接与基础路径

站点内所有链接（上 / 下一篇、文档互链、资源）都生成为**根相对地址**，例如 `/guide/foo/`，
保证任何页面深度下地址都指向正确的路由：

- **生产构建**（`plainship build`）：若 `site.url` 含路径（如 GitHub Pages 项目页
  `https://user.github.io/repo`），链接自动带上该前缀（`/repo/guide/foo/`）；
  部署在域名根路径时前缀为空
- **开发模式**（`plainship dev`）：始终使用根路径，与本地预览服务器一致
- dev 构建后需要 `plainship build` 重新构建再发布，`publish` 会校验构建产物
  与生产基础路径一致，防止 dev 产物被发布

主题模板中生成链接请使用 `url` 模板函数：`{{url .Route}}` 或 `{{url .Prev.Route}}`，
自动带上基础路径前缀。
