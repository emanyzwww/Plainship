# 架构与开发

> 本文面向开发者：模块结构、依赖方向、设计原则与路线图。
> 构建 / 测试命令见 [README「开发」](../README.md#开发)。

CLI 只做参数解析与展示，业务逻辑全部在 Core，未来可复用给 GUI / IDE 插件 / HTTP API。

## 模块结构

```
cmd/plainship  入口
internal/
├── cli/        CLI 命令（cobra，双语输出）
├── core/       核心编排（CreateSpace / Build / Publish / Status / Dev，按命令拆分）
├── revision/   版本语义：类别划分、内容指纹、机器提交协议、ps-N 编号
├── layout/     目录与文件名常量（docs / themes / build / .plainship）
├── i18n/       多语言：语言检测 + zh/en 消息表 + 主题文案注入
├── model/      核心数据模型（与主题共享）
├── space/      Space 创建与加载（Git 感知）
├── config/     配置加载与保存
├── git/        Git CLI 安全封装（通用命令）
├── parser/     Front Matter + Markdown 解析（goldmark）
├── router/     路由解析（文件名 / slug / URL 解耦）
├── builder/    增量构建管线（语言感知渲染）
├── theme/      主题系统（layouts + assets，内嵌默认主题）
├── manifest/   构建清单
├── state/      Plainship 内部状态（.plainship）
├── sync/       同步协议客户端
├── server/     Plainship Server（按 handler 拆分）
├── dev/        开发模式：文件监听 watcher + SSE 热更新服务器
├── hash/       内容哈希与构建输入哈希
└── fsutil/     安全文件系统工具
```

## 依赖方向

`layout / model / hash`（最底层，无依赖）→ `i18n / fsutil / git` →
`config / space / parser / router / theme / revision` →
`builder / manifest / state / sync` → `server / dev` →
`core`（编排）→ `cli`（最薄壳）

未来模块落点：`rollback` 进 `core` + `revision`，搜索与 RSS 进 `builder`，
插件系统作为 `builder` 的扩展点，多站点路由进 `server`。

## 设计原则

| 关注点                             | 归属                                 |
| ---------------------------------- | ------------------------------------ |
| 源文件历史、变更、协作             | **Git**（Plainship 不重复实现）      |
| 构建缓存、映射、manifest、同步状态 | **Plainship State**（`.plainship/`） |
| 解析、渲染、构建                   | **Plainship Core**（客户端）         |
| 存储、同步、静态 HTTP              | **Plainship Server**                 |

- 英文优先：工具语言默认英文（`PLAINSHIP_LANG` / `--lang` 可切换中文），站点语言由 `site.language` 控制（默认 `en`，可设为 `zh-CN`）
- 服务器永远不做 Markdown 编译 / SSR / 数据库
- 所有复杂计算（Markdown、主题、未来 Vue / React、搜索、SEO）都在客户端完成

## 路线图

- [ ] `plainship rollback <编号>` 回滚命令（内部：切换 tag 源码 + 重建）
- [x] `plainship dev` 开发模式（watch + live reload，SSE 热更新）
- [ ] `plainship dev` 增强：增量预热 / 错误覆盖层 / 自动打开浏览器
- [ ] 主题与布局系统增强（自定义组件、shortcode）
- [ ] Vue / React / Web Components（全部客户端构建）
- [ ] 插件系统（Parser / Renderer / Theme / Build / Command）
- [ ] 搜索索引 `search.json`（浏览器本地搜索）
- [ ] RSS、多站点、服务器 rollback API
- [ ] 服务器基于 Host 的多站点路由
