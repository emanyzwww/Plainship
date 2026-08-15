# 架构与开发

> 本文面向开发者:模块结构,依赖方向与开发流程.
> 设计原则与产品路线图见 [构建与发布](publishing.md).
> 构建 / 测试命令见 [README"Development"](../README.md#development).

CLI 只做参数解析与展示,业务逻辑全部在 Core,未来可复用给 GUI / IDE 插件 / HTTP API.

## 两个二进制入口

自 v0.1.0 起 Plainship 拆分为两个独立的可执行文件,**共享同一个版本号**(internal/version),
同一 tag 发布,版本一一对应:

| 二进制             | 入口                   | 职责                                                                             |
| ------------------ | ---------------------- | -------------------------------------------------------------------------------- |
| `plainship`        | `cmd/plainship`        | 客户端:Space 管理 + 构建 + 发布 + 本地预览(**不含任何服务端代码**)               |
| `plainship-server` | `cmd/plainship-server` | 服务端:存储 + 同步 + 静态 HTTP(serve / token / version,**不含任何构建发布代码**) |

安装时按需选择:服务器上只装 `plainship-server`,客户机上只装 `plainship`.

## 模块结构

```
cmd/plainship        客户端入口(构建 + 发布)
cmd/plainship-server 服务端入口(serve + token + version)
internal/
├── ui/          统一输出入口(UI 接口 + 事件流 + terminal/plain/json 渲染器 +
│                进度条/Spinner + 交互 + slog 日志投影;全部命令与 core/builder 的唯一输出通道)
├── clifx/       CLI 框架共享件:工具语言检测 / --lang 预扫描 / Windows 控制台编码
├── cli/         客户端命令(cobra,双语输出;new/create/build/publish/connect/status/dev/preview/config/version)
├── servercli/   服务端命令(cobra,双语输出;serve/token/version + 令牌文件管理 + 访问日志)
├── core/        核心编排(CreateSpace / Build / Publish / Status / Dev,按命令拆分)
├── revision/    版本语义:类别划分,内容指纹,机器提交协议,ps-N 编号
├── layout/      目录与文件名常量(docs / themes / build / .plainship)
├── i18n/        多语言:语言检测 + zh/en 消息表 + 主题文案注入
├── model/       核心数据模型(与主题共享)
├── space/       Space 创建与加载(Git 感知)
├── config/      配置加载与保存
├── git/         Git CLI 安全封装(通用命令)
├── parser/      Front Matter + Markdown 解析(goldmark)
├── router/      路由解析(文件名 / slug / URL 解耦)
├── builder/     增量构建管线(语言感知渲染,进度条反馈)
├── theme/       主题系统(layouts + assets,内嵌默认主题)
├── manifest/    构建清单
├── state/       Plainship 内部状态(.plainship)
├── protocol/    同步协议类型(Request/Response/FilePayload/ProtocolVersion,两端共享)
├── sync/        同步协议客户端(仅客户端二进制包含)
├── server/      Plainship Server 逻辑(存储 + 同步 + 静态 HTTP + slog 运行日志)
├── dev/         开发模式:文件监听 watcher + SSE 热更新服务器
├── hash/        内容哈希与构建输入哈希
└── fsutil/      安全文件系统工具
```

## 依赖方向

`layout / model / hash`(最底层,无依赖)→ `i18n / fsutil / git / style / format / clifx` →
`config / space / parser / router / theme / revision / protocol` →
`builder / manifest / state / sync` →
`ui`(依赖 style/format/i18n,输出与日志的唯一出口)→ `server / dev` →
`core`(编排,接收 `ui.UI` 作为输出入口)→ `cli`(客户端命令)/ `servercli`(服务端命令)→
`cmd/plainship` / `cmd/plainship-server`(最薄壳)

两端二进制只共享无业务含义的基础包与协议类型:

- 服务端(`cmd/plainship-server`)= `servercli + server + protocol + ui` + 基础包
- 客户端(`cmd/plainship`)= `cli + core + builder + sync + dev + ui` + 基础包

> 输出统一走 `internal/ui`(见 [输出与体验架构](output-architecture.md)):
> 命令层只表达意图(Info/Success/Warn/Detail/Progress...),颜色/对齐/进度/交互/日志
> 全部由 ui 渲染器统一处理,非终端自动降级(无色/静默进度/交互放行),`--json` 输出机器可读事件流.

未来模块落点:`rollback` 进 `core` + `revision`,搜索与 RSS 进 `builder`,
插件系统作为 `builder` 的扩展点,多站点路由进 `server`.

> 设计原则与产品路线图见 [构建与发布](publishing.md).

## 贡献

欢迎通过 [Issues](https://github.com/emanyzwww/plainship/issues) 报告问题,提出建议,或提交 Pull Request:

1. Fork 本仓库并创建功能分支
2. 修改后运行 `go test ./...` 确保测试通过
3. 提交 PR,说明改动动机与影响

建议:涉及协议或行为变更时,同步更新 [docs/](.) 中的对应文档;中文交流优先.