# 构建与发布

> 本文解释 Plainship 的构建 / 提交 / 编号机制,Release 与回滚原理,以及设计理念.
> 具体操作见 [使用指南](usage.md),快速上手见 [README](../README.md).

## 核心工作流

```text
Git 负责:  内容 · 历史 · Diff · Branch · Collaboration · Recovery
Plainship: Build → Preview → Publish → Release → Rollback → Serve
静态文件:  Runtime
```

- **Git owns the content.**
- **Plainship owns publishing.**
- **Static files own the runtime.**

Plainship 不是又一个 Markdown 静态网站生成器,而是一层 **Publishing Layer**:把 Git 里已有的内容变成可发布的网站.

## 为什么使用 Plainship

- **Git-native**:内容就是 Git 仓库里的 Markdown,没有私有格式
- **Local-first**:一切在本地完成,编辑器 / 终端 / Git 工作流不变
- **一条命令发布**:`plainship build` 增量构建 + 自动提交 + 打构建编号;`plainship publish` 同步到服务器
- **静态优先**:`build/` 是完整独立的静态网站,不依赖数据库 / CMS / 服务器端渲染
- **可回滚**:每次构建有编号(`ps-N` tag),还原源码 + 重建即可回滚
- **可迁移**:不用 Plainship 后,Markdown + Git 依然可用;静态 HTML 可部署到任意托管
- **极小**:两个小型静态二进制(客户端 + 服务端),无运行时依赖

## Git 是 Source of Truth

Plainship 不替代 Git,也不创建第二个事实来源:

- `docs/`,`themes/`,`plainship.yaml` 全部进 Git
- `plainship build` 自动分步提交(config / theme / docs 三类),提交信息为机器格式
- 构建编号记录为 Git tag(`ps-0001`,`ps-0002`...),历史天然保留在 Git 中
- `build/` 与 `.plainship/` 不进 Git:它们可以由源码 + Plainship 版本复现

## 一条命令发布

```bash
plainship publish
```

`publish` 会先校验(不满足则拒绝发布,绝不发布半成品):

1. 当前源码无未提交变更(config / theme / docs 均 clean)
2. `build/` 必须由当前源码构建(类别指纹一致)
3. `build/` 必须由生产构建产生(防止 dev 产物被发布)
4. 渲染器版本与当前二进制一致(防止升级后发布旧渲染)

校验通过后增量上传差异文件 → 服务器原子激活(上传 → 校验 → 切换 `current` 指针,绝无半发布状态).

## Release / Version

每次成功构建都是一个 **Release**:

- 构建编号:`ps-0001`,`ps-0002`...(Git tag,指向该次构建的最后一个提交)
- 机器提交协议:`<类别> build=<编号> hash=<内容指纹16位>`,可解析,可校验
- 服务器端:每次同步在 `data/sites/<siteId>/releases/<buildId>/` 保存一份 **完整快照**,并记录构建元数据(`release.json`)
- 增量发布:客户端只上传差异文件,服务器基于上一版本补齐,保证每个 release 都是完整快照

构建输入 = docs + themes + config + Plainship 版本,因此构建可复现.

## Artifact Model

```text
源码 (docs + themes + plainship.yaml)
      ↓ plainship build
静态网站快照 (build/)
      ↓ plainship publish
服务器 Release (data/sites/<siteId>/releases/<buildId>/)
      ↓ 激活
线上版本 (current.json 指针 → 静态 HTTP)
```

## Portability

- **内容可迁移**:不依赖 Plainship 的私有数据格式,换用任何 Markdown 工具链都可行
- **产物可迁移**:静态 HTML 可部署到 GitHub Pages,Nginx,任意对象存储或静态托管
- **服务器可替换**:即使 Plainship Server 消失,已发布的静态站点不受影响

## 设计原则

| 关注点                          | 归属                               |
| ------------------------------- | ---------------------------------- |
| 源文件历史,变更,协作            | **Git**(Plainship 不重复实现)      |
| 构建缓存,映射,manifest,同步状态 | **Plainship State**(`.plainship/`) |
| 解析,渲染,构建                  | **Plainship Core**(客户端)         |
| 存储,同步,静态 HTTP             | **Plainship Server**               |

## Roadmap

- [ ] `plainship rollback <编号>` 回滚命令(内部:切换 tag 源码 + 重建)
- [x] `plainship dev` 开发模式(watch + live reload,SSE 热更新)
- [ ] `plainship dev` 增强:增量预热 / 错误覆盖层 / 自动打开浏览器
- [ ] 主题与布局系统增强(自定义组件,shortcode)
- [ ] 搜索索引 `search.json`(浏览器本地搜索)
- [ ] RSS,服务器端 rollback API,基于 Host 的多站点路由

## Project Status

核心链路(创建 → 写作 → 构建 → 发布 → 静态访问 → Git 回滚)已可用.协议与 CLI 仍在演进,请以实际命令与文档为准.

## Plainship 不是什么

Plainship 刻意不包含:数据库 / CMS Runtime / 服务器端渲染,在线编辑器 / 评论系统 / 用户系统 / 权限系统,Analytics / 插件市场 / SaaS / Billing,AI 写作 / Realtime Collaboration.内容与历史属于 Git,运行时属于静态文件,Plainship 只负责构建与发布这条边界.
