# Plainship 输出与体验架构 (v2)

> 状态: **已实施** (2026-02, 阶段 0-5 全部落地, 全量测试通过)
> 版本: 2026-02 (替代 v1 草案; 本文同时是 internal/ui 的包设计文档)
> 目标: 输出源可扩展 / 终端体验规范化 / 客户端与服务端统一 / 日志就位 / 机器可读输出

## 1. 项目理解: Plainship 是什么

Plainship 是一层 **Publishing Layer**(发布层): 把 Git 仓库里的 Markdown 内容变成可发布的静态网站.
它不是又一个静态站点生成器, 而是承接 "Git 拥有内容 → Plainship 拥有发布 → 静态文件拥有运行时" 这条边界.

三个核心工作流, 每个都有独立的输出场景:

| 工作流 | 命令                          | 输出场景特征               |
| ------ | ----------------------------- | -------------------------- |
| 内容流 | new → create → build → status | 一次性命令, 结果展示型输出 |
| 发布流 | connect → publish             | 含网络等待与交互输入       |
| 运行流 | dev / preview / serve         | 长驻进程, 持续输出型       |

关键约束 (方案必须满足):

- **两个二进制共享框架件**: 客户端 (cmd/plainship) 与服务端 (cmd/plainship-server)
  共享 internal/{i18n,style,format,clifx}, 输出行为必须两端一致.
- **CLI 是薄壳**: docs/architecture.md 明确 "CLI 只做参数解析与展示, 业务逻辑全部在 Core,
  未来可复用给 GUI / IDE 插件 / HTTP API" —— 这意味着**输出必须是与业务解耦的事件流**,
  而不是散落在业务代码里的打印调用.
- **双语**: 工具语言 zh/en (PLAINSHIP_LANG / --lang / 配置), 消息模板用 `{{ .arg0 }}` 命名变量.
- **三平台**: Windows (GBK 控制台 → UTF-8 切换, VT 启用) / Linux / macOS.
- **零依赖偏好**: go.mod 仅 4 个直接依赖; 方案不引入第三方输出/日志库.
- **测试策略**: 现有测试用 `root.SetOut/SetErr` 注入 buffer 断言输出, 必须保留.

## 2. 现状问题清单 (通读证据)

### 2.1 输出实现分散 (四层重复)

1. `clifx.Printf/Println` (internal/clifx/clifx.go) — servercli 直调约 20 处.
2. `cli` 包内 `printf/println` 薄封装 (internal/cli/io.go) — cli 内约 67 处.
3. core/builder 内 4 份复制粘贴的局部闭包 `printl`/`printls`/`log`
   (core/build.go:38, core/dev.go:34, core/publish.go:33, builder/builder.go:73), 逐字相同.
4. 直连旁路: `fmt.Fprintln(os.Stderr, ...)` (space/space.go:93-95 Git 缺失警告,
   core/dev.go:73 服务器错误), `fmt.Fprint` 交互提示 (cli/publish.go:79, cli/connect.go:46).

### 2.2 终端体验不一致 (任务 A 发现, 具体到命令)

1. **对齐不统一**: 仅根帮助用 format 包 (cli/root.go:64), 其余全部手写两格空格字面缩进
   (status.go:35, build.go:134 等); 中英混排时 CJK 2 列宽导致错位.
2. **配色不一致**: status/new/connect/serve 有色; build/publish/dev **完全无色**
   (publish 整条命令无一处 style); 颜色 API 三种风格并存 (整行 Green / 局部 st.Cyan 嵌模板 / 无).
3. **缺进度反馈**: build 只有 Scanning/Building 两行; publish 网络阶段 (60s 超时) 只有一行
   "Publishing..."; 大站点解析/渲染期无任何进度.
4. **交互不规范**: connect 令牌输入明文回显且无终端判据 (脚本场景直接挂起或报错);
   publish 有 IsTerminal 放行而 connect 没有; 两处 bufio 交互逻辑重复.
5. **错误通道归类不一致**: 顶层错误走 stderr (RenderError), 但 dev 服务器异步错误、
   space Git 警告走裸 os.Stderr, 且无颜色、无建议.
6. **无日志**: 标准库 log/slog 零使用; 服务端无 HTTP 访问日志, dev 无重建日志, 排障只能肉眼.

### 2.3 结构性事实 (决定方案形态)

- build 的慢段是解析 (builder.go 阶段⑥) 与逐页渲染 (阶段⑨, 当前每页打印一行 `✓ 标题`),
  文件总数在扫描后已知 → **可以做成真实进度条** (i/N).
- publish 是单次原子 HTTP POST (所有文件 base64 打包, sync/client.go:200), 无逐文件上传
  → 网络阶段只能做 **spinner + 字节数/文件数** 反馈, 不能做逐文件进度条.
- dev 是长驻进程 (watcher 300ms 轮询 + SSE), 每次重建输出一组信息 → 适合 **状态行** (单行刷新).
- serve 的启动横幅已有参考模板 (scripts/install.sh:280-298 框线 + 键值对齐 + 绿色 token).
- config 包已预留 `color` 配置项 (GlobalClient.Color/SpaceClient.Color, 默认 true)
  与运行时 flag 层 (config/config.go:46-59) → 输出开关应接入现有配置链, 而不是新造一套.

### 2.4 服务端现状 (任务 C 发现)

- **生产 server 完全零日志**: 无 HTTP 访问日志 (server.go:62 裸 `http.ListenAndServe`),
  无启动/同步/激活日志; 排障只能靠客户端错误信息.
- serve 强制 Token 非空才启动 (server.go:58-63), 认证永远开启.
- 同步协议: 单次 POST 全量 payload (base64), 服务器原子激活 (tmp+rename current.json),
  增量继承上一 release 保证快照完整 (sync.go:71-79).
- 长耗时点: 客户端 Diff 全量哈希 (client.go:262-273)、请求打包、60s HTTP 超时;
  服务端 io.ReadAll 整读 (上限 1GB)、base64 解码、CopyDir 继承.

## 3. 设计目标

1. **输出源可扩展**: 终端 / 文件 / 网络 / 多路复用, 命令层零改动.
2. **终端体验统一规范**: 颜色语义、对齐、区块、进度、交互全部收敛到单一实现.
3. **客户端与服务端统一**: 同一套 `ui` 包, 两个二进制行为一致.
4. **日志就位**: 标准库 slog, 事件流双投影; 服务端补 HTTP 访问日志.
5. **机器可读**: `--json` 全局标志, 脚本/CI 可消费.
6. **零新依赖 + 保留测试策略**.

## 4. 架构: 事件流 + 渲染器 + 传输层

### 4.1 核心模型

一次输出 = 产生一个**结构化事件** → 由**渲染器**投影到任意 **sink**.

```
命令层 (cli / servercli)                只依赖 ui.UI
   │
   ▼
internal/ui: UI 接口 → 事件流 Event      输出什么 (结构化)
   │
   ├──► 渲染器 (怎么显示)
   │      terminal: ANSI 色板 + 对齐 + 进度条 + 交互
   │      plain:    无色 + 进度退化 + 交互自动放行
   │      json:     事件序列化 (机器可读)
   │
   └──► slog 日志投影 (怎么记录)
          Info/Warn/Error/Debug 级别
   │
   ▼
Sink (写到哪): stdout / stderr / 文件 / 网络 / MultiWriter
```

### 4.2 事件模型 (internal/ui/event.go)

```go
type Level int
const (
    LevelInfo Level = iota  // 用户可见普通信息 (stdout)
    LevelSuccess            // 成功 (终端绿色)
    LevelWarn               // 警告 (stderr, 终端黄色)
    LevelError              // 错误 (stderr, 终端红色 + 建议)
    LevelDebug              // 诊断 (仅日志投影, 不进用户输出)
)

// Arg 是命名参数: 终端渲染成 "键: 值" (自动对齐), JSON 渲染成字段.
type Arg struct{ Key, Value string }

// Event 是渲染器与日志共用的结构化事件.
type Event struct {
    Level  Level
    Text   string      // 已通过 i18n.T 渲染的文案 (渲染层不感知语言)
    Args   []Arg       // 结构化字段 (Detail/Table 的对齐依据, JSON 的字段)
    Time   time.Time
}
```

要点: 事件层不感知语言 (文案在调用点已由 i18n 渲染), 不感知颜色 (由渲染器决定),
只承载 "级别 + 文案 + 字段".

### 4.3 UI 接口 (命令层唯一输出 API)

```go
package ui

type UI interface {
    // —— 结果输出 (默认 stdout) ——
    Info(text string)                       // 普通信息
    Success(text string)                    // 成功 (终端: 绿色)
    Warn(text string)                       // 警告 (stderr, 终端: 黄色)
    Error(err error)                        // 错误 (stderr, 红色 + 建议, 复用 clifx.SuggestFor)
    Debug(text string)                      // 诊断 → 仅日志投影

    // —— 结构化展示 ——
    Detail(label, value string)             // 键值对, 自动对齐 (format 包, CJK 宽度)
    Table(headers []string, rows [][]string) // 表格, 自动对齐
    Section(title string)                   // 区块标题 (终端: 粗体 + 上下空行)

    // —— 进度 (终端独占; 非终端静默, 调用方负责最终摘要行) ——
    Spinner(text string) func(done string)  // 旋转指示器; 结束回调可传完成文案 (done 非空时输出该行)
    Progress(total int) *Progress           // 从左到右进度条; Set(n, label) 推进

    // —— 交互 (stdin/stdout 非终端时自动放行或取默认) ——
    Confirm(prompt string) bool             // y/yes 确认 (非终端返回 true, 自动化不被挂起)
    Prompt(label string, secret bool) (string, error) // 单行输入; secret=true 不回显; 非终端返回 ErrNonInteractive

    // —— 收尾与逃生口 ——
    Flush()                                 // 立即输出挂起的 Detail (命令以 Detail 收尾时调用)
    Writer() io.Writer                      // 底层 stdout writer (SSE 直写等, 尽量少用)
}
```

**接口即规范**: 调用方只能表达意图 (Success/Detail/Progress), 无法绕过对齐与颜色.
core/builder 接收 `ui.UI` (`nil` 保留现状静默语义), 不再是 `io.Writer`.

### 4.4 渲染器与自动降级链

```
--json 显式指定  → JSON 渲染器: 每事件一行 JSON (level/time/msg/args), 无颜色无交互
输出是 TTY      → Terminal 渲染器: ANSI + 对齐 + 进度条 + 交互
输出非 TTY      → Plain 渲染器: 无色; 进度退化为行; Confirm 放行; Prompt 报错提示
```

- 判定复用 `style.enabled` 与 `style.IsTerminal` (NO_COLOR / --no-color / Windows VT 已内置).
- UI 构造: `ui.New(ui.Options{Out, Err, In, Format, Logger})`, 默认 Out=stdout, Err=stderr,
  Format 自动判定; 测试注入 bytes.Buffer 与现状 `SetOut/SetErr` 等效.

### 4.5 日志投影 (internal/ui/slog.go)

原则: **事件流是唯一真相, UI 输出与日志是它的两个投影** —— 不会出现两套文案双维护.

| UI 方法        | slog 级别 | 说明                                              |
| -------------- | --------- | ------------------------------------------------- |
| Info / Success | Info      | 用户可见输出也记录, 事后可审计                    |
| Warn           | Warn      | 警告双写                                          |
| Error          | Error     | 错误双写                                          |
| Debug          | Debug     | 仅 --verbose / 日志级别允许时可见                 |
| (HTTP 访问)    | Info      | 服务端中间件单独产生: method path status duration |

slog Handler 选择: 默认 TextHandler (stderr), `--log-format=json` 时 JSONHandler,
`--log-file` 时写入文件. 零第三方依赖.

### 4.6 全局标志与配置链

接入现有 cobra 根命令 (cli/root.go 与 servercli/root.go 的 PersistentFlags):

| 标志               | 默认                          | 作用                   |
| ------------------ | ----------------------------- | ---------------------- | ---- |
| `--lang zh         | en`                           | 配置链                 | 已有 |
| `--no-color`       | 已有                          | 已有                   |
| `--json`           | false                         | JSON 渲染器 (机器可读) |
| `--verbose` / `-v` | false                         | 打开 Debug 投影        |
| `--log-level`      | warn (客户端) / info (服务端) | 日志级别               |
| `--log-file`       | "" (stderr)                   | 日志文件               |
| `--log-format`     | text                          | text / json            |

环境变量沿用现有约定: `PLAINSHIP_JSON`, `PLAINSHIP_VERBOSE`, `PLAINSHIP_LOG_LEVEL` 等
(键名大写 + PLAINSHIP\_ 前缀, 与 config/sources.go:17 的规则一致; 或直接注册进 config 的
runtime 层, 复用 Config.Color 同款机制).

### 4.7 包结构与依赖方向

```
internal/ui (零第三方依赖, 依赖 style/format/i18n)
  ├── event.go              事件模型 (Level / Event)
  ├── ui.go                 UI 接口 + Options + 实现 (事件分发/Detail 缓冲/时间戳)
  ├── renderer.go           渲染格式与分派 (Format / resolveFormat / renderLine)
  ├── renderer_terminal.go  终端渲染 (ANSI 颜色判定)
  ├── renderer_plain.go     纯文本渲染契约 (无色/进度静默/交互放行)
  ├── renderer_json.go      JSON 事件流渲染 (jsonEvent / writeJSON)
  ├── slog.go               日志投影 (事件 → slog 双投影)
  ├── progress.go           Progress / Spinner 状态机 (terminal 专用)
  ├── interact.go           交互 (Confirm / Prompt / secret 回显控制)
  ├── term_linux.go         平台终端回显控制 (Linux)
  ├── term_darwin.go        (macOS)
  ├── term_windows.go       (Windows)
  ├── term_other.go         (其他平台兜底)
  ├── suggest.go            错误建议映射 + 错误渲染 (RenderError)
  ├── mark.go               样式标记系统 (Green/Yellow/Cyan/Bold/Dim/Red + RenderMarks)
  ├── ui_test.go            单元测试
  └── suggest_test.go       建议映射测试
```

依赖方向: ui 位于 `style/format/i18n` 之上, `core/builder` 之下,
即 `… → style/format/i18n → ui → core → cli/servercli → cmd`.
错误建议映射 (suggest) 从 clifx 迁入 ui, clifx 仅保留语言检测与控制台编码.

## 5. 终端体验规范

### 5.1 语义色板 (固定, 复用 style)

| 颜色 | 语义   | 用途                           |
| ---- | ------ | ------------------------------ |
| 绿   | 成功   | 完成提示, clean 状态, ✓ 行     |
| 黄   | 警告   | 需要注意, 未构建, 交互降级提示 |
| 红   | 错误   | 错误消息 (RenderError 现行为)  |
| 青   | 关键值 | URL, 构建编号, token           |
| 粗体 | 标题   | 版本头, Section 标题           |
| 暗   | 次要   | 辅助说明                       |

规则: **整行着色优先, 参数内嵌着色仅限关键值** (统一为 st.Cyan 嵌 Detail 值的风格,
消灭 "整行 Green" 与 "完全无色" 并存).

### 5.2 组件规范

- **Section(title)**: 粗体标题 + 上下空行. 用于 status 的 Space/Git/Build/Publish 区块,
  new 的目录结构/下一步, serve 的启动信息.
- **Detail(label, value)**: 冒号对齐 (format.displayWidth, CJK 2 列), 例:
  ```
  Server URL: http://0.0.0.0:9090
  Data dir:   ./data
  Sites:      (none)
  ```
  替换全部手写两格缩进 (status.go:35, serve.go:61-74 等).
- **Table(headers, rows)**: 自动对齐, 用于变更统计 (config/theme/docs × added/modified/deleted).
- **Progress(total)**: 从左到右进度条 `▰▰▰▰▱▱▱ 8/12`, 非终端退化为 `Building 8/12...` 行.
  用于 build 解析/渲染阶段 (i/N 文档, builder.go 阶段⑥⑨的 `✓ 标题` 行升级为进度条,
  完成时输出摘要行).
- **Spinner(text)**: `⠋ Publishing...`, 用于 publish 网络阶段 (附文件数/字节数),
  指纹哈希等无总量阶段; 非终端退化为 `Publishing...` 行.
- **状态行 (dev/serve 长驻)**: 单行 `\r` 重写 + 时间戳前缀 `[12:00:05]`,
  用于 dev 重建事件与 serve 访问日志; 非终端退化为逐行日志.

### 5.3 交互规范

- Confirm: 仅 stdin+stdout 均终端时交互, 否则放行 (现状 publish 行为, 收敛为组件).
- Prompt(secret): connect 令牌输入升级为 secret 模式 (不回显, 终端可用 `stty -echo` /
  Windows `SetConsoleMode` 关闭回显); 非终端时: 有 --token 用 flag, 否则报错并提示.
- 统一实现放行/降级逻辑, 消灭 connect.go:47 与 publish.go:80 的重复.

### 5.4 错误规范

```
错误: <红色消息>            (i18n 渲染, 含原因链)
建议: <黄色建议>            (clifx.SuggestFor 映射, 无匹配则不输出)
(--verbose 时追加: 错误类型 / 内部错误链 / 文件:行号)
```

统一出口: 命令返回 error → 顶层 RenderError (现状 cli/root.go:79, servercli/root.go:63).
消灭旁路: space.go:93-95 的 Git 警告 → Warn 组件 (stderr + 黄色);
core/dev.go:73 的服务器错误 → ui.Error 或日志 (按级别).

## 6. 命令输出矩阵 (目标设计)

### 6.1 build (结果展示 + 进度)

```
Plainship v0.1.0                         [Bold]
Branch: main                            [Detail]

config    +1 added   ~1 modified        [Table: 类别 × 变更]
theme     clean                            [Green]
docs      +3 added                         [Table: 类别 × 变更]

Building pages  ▰▰▰▰▰▰▱▱▱▱  6/12       [Progress]
  • docs/guide/quickstart.md             [当前项 / Active]

Build complete                           [Bold]
  8 changed   3 copied   1 deleted       [Summary: 构建结果]

  ✓ commit  docs build=ps-0004  ab12cd34 [Success × 关键操作]
  ✓ tag     ps-0004                       [Success × 关键操作]

Release  ps-0004  ·  2026-08-13 10:00:00 [Detail, Release ID: Cyan]
```

### 6.2 publish (守卫 + 阶段反馈)

```
Plainship v0.1.0                              [Bold]

Checking publish guards                         [Section / Bold]
  ✓ source clean                                [Success / Green]
  ✓ build matches sources                       [Success / Green]
  ✓ production build                            [Success / Green]
  ✓ renderer version                            [Success / Green]

Publishing  ps-0004  ·  24 files  ·  1.2 MB      [Spinner / Network phase]
✓ Uploaded   24 files  ·  2 deleted               [Success / Green + Detail]
✓ Activated  ps-0004  ·  http://server:9090       [Success / Green, URL: Cyan]
```

### 6.3 dev (长驻状态行)

```
Plainship dev                                  [Bold]

Serving   http://localhost:8080                [Green, URL: Cyan]
Watching  docs/  themes/  plainship.yaml       [Detail]

[12:00:01]  Building...                        [Status / Active]
[12:00:03]  ✓ Rebuilt 3 pages in 1.2s           [Success / Green]
            · reload sent                       [Detail]

[12:00:09]  ✗ Build failed                     [Error / Red]
            docs/x.md · front matter            [Error Detail]
            Waiting for changes...              [Detail / Idle]

Ctrl+C      Stopped                            [Exit / Detail]
```

### 6.4 serve (启动横幅 + 访问日志)

```
Plainship Server v0.1.0                         [Bold]

Server URL  http://0.0.0.0:9090                  [Detail / URL]
Data dir    ./data                               [Detail]
Sites       my-docs                              [Detail]
Auth        Bearer token enabled                 [Green / Status]

Access token  (copy this)                        [Section / Highlight]
  ps_ab12cd34ef56ab12cd34ef56ab12cd34            [Token / Copyable]

API                                             [Section / Bold]
  POST  /api/v1/sites/{siteId}/sync             [Endpoint / Method: Cyan]
  GET   /api/v1/sites/{siteId}/status           [Endpoint / Method: Cyan]
  GET   /api/v1/sites/{siteId}/releases/{buildId} [Endpoint / Method: Cyan]

[12:00:05]  POST /api/v1/sites/my-docs/sync  200  1.4s   [Access Log / Single-line Record]
             ↑ path                           ↑    ↑
             endpoint                         status time
```

### 6.5 status (保留现有结构, 组件化)

现有 status 已是最结构化的命令, 直接映射到组件: 版本头 → Section(Space/Git/Changes/Build/Publish)
→ Detail/Table/状态色. 无行为变化, 只替换手写排版.

### 6.6 new / create / connect / config / version / token / preview

- new: Section(目录结构) + Detail(Git 状态) + Section(下一步, 4 步).
- create: Success + Detail(编辑提示).
- connect: 交互升级 (Prompt secret) + Success + Detail(下一步).
- config/version/token: Success/Detail 组件化, 无色命令补上 Success 绿勾.
- preview: 同 serve 简化横幅 + 端口预检 (补上 ListenAndServe 提前报错的问题, 参考 dev 的 net.Listen 早检).

## 7. 实施计划 (分阶段, 每阶段可编译可测试)

### 阶段 0: ui 包基础 (~500 行)

- 新建 internal/ui: event.go / ui.go / renderer.go / renderer_terminal.go / renderer_plain.go / slog.go.
- 实现 Info/Success/Warn/Error/Detail/Section/Table/Debug + Writer 逃生口.
- 渲染器选择链 (TTY/plain) + slog 投影.
- 单测: 三渲染器 golden 输出 (终端含色 / 纯文本 / JSON), 事件断言.
- 验收: `go test ./internal/ui/...` 通过; 无新依赖.

### 阶段 1: core/builder 迁移

- 签名 `out io.Writer` → `out ui.UI` (`nil` 保留静默): core.Build/core.Dev/core.Publish,
  builder.Build/BuildDev.
- 消灭 4 处 printl 闭包 → ui.Info/Success/Warn.
- stderr 旁路收敛 (space.go Git 警告 → ui.Warn; core/dev.go → ui.Error).
- 现有测试适配: 传 `ui.New(ui.Options{Out: buf})` 或保留 nil.
- 验收: 现有 cli/core/builder 测试全部通过, 输出文本与现状一致 (纯文本渲染器).

### 阶段 2: 命令层迁移

- cli 包 printf/println → ui (删除 io.go 薄封装); servercli 直调 clifx → ui.
- 交互收敛: ui.Confirm / ui.Prompt(secret); connect 补终端判据与 secret 输入.
- 根命令标志: --json / --verbose / --log-\*.
- 两处 root.go Execute() 合并逻辑 (共用 ui.NewFromRoot(cmd) 工厂).
- 验收: cli/servercli 测试全部通过; 手工验证各命令输出与现状一致或更好.

### 阶段 3: 进度与状态行

- ui.Progress / ui.Spinner (terminal 独占, plain 退化).
- build: 解析/渲染阶段进度条 (builder 暴露总页数与当前页).
- publish: 网络阶段 spinner + 文件数/字节数 (Diff 后可知).
- dev: 状态行 + 重建耗时 + 时间戳; serve: 访问日志中间件.
- 验收: TTY 与管道两种模式下输出均正确; CI 日志可读.

### 阶段 4: 服务端日志

- serve: --log-level / --log-file / --log-format; 访问日志中间件 (method path status duration,
  认证失败/路径非法记 Warn, 正常记 Info).
- 服务端日志点清单 (当前全部缺失):
  - 启动: Server vX / addr / data dir / sites (走 UI 横幅 + slog.Info 各一条)
  - HTTP 访问: `INFO method path status duration bytes` (中间件包裹 s.Routes())
  - 同步成功: `INFO sync site=<id> build=<id> files=<n> deletes=<n> full=<bool>`
  - 同步失败: `ERROR sync site=<id> err=<msg>`
  - 激活: `INFO activate site=<id> build=<id>` / 缺 index.html 等失败 `ERROR`
  - 鉴权失败: `WARN auth 401 site=<id> path=<path>`
  - dev 模式: 重建事件 `INFO rebuild ok pages=<n> dur=<ms>` / `ERROR rebuild err=<msg>`
- 客户端: --verbose 打开 Debug 投影.
- 验收: serve 访问站点后日志可见; --log-file 落盘; JSON 格式可被 jq 解析.

### 阶段 5: --json 机器可读

- JSON 渲染器全命令接入; 文档补示例.
- 验收: `plainship status --json` 输出合法 JSON 事件流.

## 8. 决策记录

| 决策                                                                    | 理由                                                               |
| ----------------------------------------------------------------------- | ------------------------------------------------------------------ |
| 事件流 + 渲染器 + sink 三层                                             | 输出源可扩展 (文件/网络 = 新渲染器或新 sink, 命令层零改动);        |
| 与 docs/architecture.md "CLI 薄壳, core 复用给 GUI/HTTP API" 的愿景一致 |
| 标准库 slog 双投影                                                      | Go 1.26 内置零依赖; 事件流是唯一真相, 日志是投影, 无双维护         |
| UI 接口而非裸 writer                                                    | 接口即规范: 对齐/颜色/进度/交互在实现层统一, 调用方无法绕过        |
| 不引入 lipgloss 等                                                      | 项目已有 style (颜色) + format (对齐); 自研 ui 包约 500 行可全覆盖 |
| 保留 nil=静默语义                                                       | 兼容现有测试传 nil / os.Stdout 的用法, 迁移期不破坏                |
| --json 全局标志                                                         | 脚本/CI 场景机器可读, 现代 CLI 标准能力 (gh/docker 同款)           |
| 进度条只做 build, publish 用 spinner                                    | build 有已知总量 (文档数); publish 是原子 POST 无法逐文件进度      |
| 服务端日志走 slog 而非 UI 行                                            | 长驻进程输出需时间戳/级别/可落盘, 用户横幅与运行日志天然分层       |
| 保留 i18n 消息模板不变                                                  | 264 键已覆盖文案; 结构化字段 (Args) 由调用点补充, 不重写消息系统   |
| config 链接入输出开关                                                   | 已有 color 配置项与 runtime 层机制, 不另造配置系统                 |

---

## 9. 实施记录 (2026-02)

阶段 0-5 全部落地, 与本文档的差异与补充:

| 项目 | 落地情况 |
|---|---|
| `internal/ui` 包 | 已实现 (文件结构见 4.7 更新版), 零第三方依赖 |
| core/builder/space 签名 | `io.Writer` → `ui.UI` (`nil` → `ui.Discard` 静默单例), 消灭 4 处 printl 闭包 |
| cli/servercli 迁移 | 56 处打印调用收敛到 ui; 删除 `internal/cli/io.go` 薄封装; publish 确认与 connect 令牌输入收敛到 `ui.Confirm` / `ui.Prompt(secret)` (令牌不再明文回显) |
| 死代码清理 | `clifx.Printf/Println/RenderError/SuggestFor` 删除; 建议映射迁入 ui (suggest.go); 测试同步迁移 |
| 第 6 章样张 | build/publish/dev/serve/status/new/config/preview 全部按样张实现 (见 §6) |
| 全局标志 | `--json` / `--verbose` (两端); `--log-level` / `--log-file` / `--log-format` (serve) |
| serve 运行日志 | HTTP 访问日志中间件 (method/path/status/duration) + 同步/激活/鉴权事件日志 |
| 平台支持 | term_linux/darwin/windows/other 四文件 (darwin 用 TIOCGETA/TIOSETA, 修复交叉编译) |

实施中修正的设计问题:

1. **嵌套标记**: `RenderMarks` 按配对深度解析, 支持 `Green(Cyan(url))` 任意嵌套 (早期实现找首个闭合符会截断内容).
2. **Summary 死锁**: Summary 曾持锁调用 emit (Mutex 不可重入) 导致 build 命令挂死, 已改为 emit 自行加锁.
3. **Section 空行**: emit 拆分后曾漏设 wrote 标记, 已补.
4. **时间戳前缀**: `[HH:MM:SS]` 带方括号 (与 §6.3 样张一致).
