# Plainship

> [English](README.md) | **中文**

> **Plainship — 面向 Git 原生内容的极简发布系统。**
> **Plainship — ship your Git-native content as durable static websites.**

你已经把内容放在了 Markdown + Git 里。Plainship 解决的是最后一步：

```text
Markdown + Git
      ↓
  Plainship
      ↓
Static Artifact
      ↓
   Publish
      ↓
Static Website
```

一个目录 + Markdown + Git + 一个命令 = 一个可发布、可回滚的静态网站。

![Go](https://img.shields.io/badge/go-1.26.5-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-lightgrey)

## Plainship 解决什么问题

Plainship 不是又一个 Markdown 静态网站生成器。它是一层 **Publishing Layer**，负责把 Git 里已经存在的内容变成**可发布的网站**：

- 用你喜欢的编辑器维护本地 Markdown 文件（VS Code / Vim / Neovim）
- 内容历史、Diff、分支、协作、恢复全部交给 Git（Plainship 不重复实现版本控制）
- 一个命令增量构建为完整静态网站，并作为**不可变的 Release** 发布
- 服务器只做存储 + 同步 + 静态 HTTP：无数据库、无 CMS Runtime、无服务器端构建

## 核心工作流

```text
Git 负责:  内容 · 历史 · Diff · Branch · Collaboration · Recovery
Plainship: Build → Preview → Publish → Release → Rollback → Serve
静态文件:  Runtime
```

- **Git owns the content.**
- **Plainship owns publishing.**
- **Static files own the runtime.**

## 为什么使用 Plainship

- **Git-native**：内容就是 Git 仓库里的 Markdown，没有任何私有格式
- **Local-first**：全部构建在本地完成，编辑器、终端、Git 工作流保持不变
- **一条命令发布**：`plainship build` 增量构建 + 自动提交 + 打构建编号；`plainship publish` 同步到服务器
- **静态优先**：`build/` 是完整独立的静态网站，不依赖数据库、CMS 或服务器端渲染
- **可回滚**：每次构建都有编号（`ps-N` tag），还原源码 + 重建即可回滚
- **可迁移**：即使未来不用 Plainship，Markdown + Git 依然可以用其他工具；静态 HTML 可以部署到任意托管平台
- **极小**：单个静态二进制，无运行时依赖

## Git 是 Source of Truth

Plainship 不替代 Git，也不创建第二个事实来源：

- `docs/`、`themes/`、`plainship.yaml` 全部进 Git
- `plainship build` 自动分步提交（config / theme / docs 三类），提交信息为机器格式
- 构建编号记录为 Git tag（`ps-0001`、`ps-0002`……），历史天然保留在 Git 中
- `build/` 与 `.plainship/` 不进 Git：它们可以由源码 + Plainship 版本**复现**

## 一条命令发布

```bash
plainship publish
```

`publish` 会先校验（不满足则拒绝发布，绝不发布半成品）：

1. 当前源码无未提交变更（config / theme / docs 均 clean）
2. `build/` 必须由当前源码构建（类别指纹一致）
3. `build/` 必须由生产构建产生（防止 dev 产物被发布）
4. 渲染器版本与当前二进制一致（防止升级后发布旧渲染）

校验通过后增量上传差异文件 → 服务器原子激活（上传 → 校验 → 切换 `current` 指针，绝无半发布状态）。

## Release / Version

每次成功构建都是一个 **Release**：

- 构建编号：`ps-0001`、`ps-0002`……（Git tag，指向该次构建的最后一个提交）
- 机器提交协议：`<类别> build=<编号> hash=<内容指纹16位>`，可供程序解析与校验
- 服务器端：每次同步在 `data/sites/<siteId>/releases/<buildId>/` 保存一份**完整快照**，并记录构建元数据（`release.json`）
- 增量发布：客户端只上传差异文件，服务器基于上一版本补齐，保证每个 release 都是完整快照

## Rollback

回滚的本质是「还原源码 + 重新构建」：

```bash
git checkout ps-0003     # 还原到某个 Release 的源码
plainship build
plainship publish        # 重新发布
```

- 单篇回滚：`git log -- docs/某篇.md` → `git checkout ps-0003 -- docs/某篇.md` → `plainship build`
- 整站回滚：`git checkout ps-0003` → `plainship build` → `plainship publish`
- 服务器保留全部 release 快照；服务器端 rollback API 在路线图中

## Local Development

```bash
plainship dev
```

监听 `docs/`、`themes/` 与 `plainship.yaml` 变更，自动重建并通过 SSE 热更新浏览器（默认 `:8080`）。dev 模式只构建，不提交 Git、不打编号。

## Static Output

`build/` 是一个完全独立的静态网站，可以脱离 Plainship Server 部署：

```bash
cd build
python -m http.server 8000   # 或任意静态服务器 / Nginx / GitHub Pages
```

站点内所有链接都生成为**根相对地址**，任意页面深度与任意部署方式下地址都正确。

## Self-hosted Server

```bash
plainship serve --addr :9090 --data ./data
```

服务器只做三件事：**存储 + 同步 + 静态 HTTP**。

- 无数据库、无 Node.js、无构建依赖
- 认证永远开启（Bearer Token，自动生成并持久化到 `data/server.token`）
- 原子发布：上传 → 校验 → 激活 `current` 指针
- 构建版本可回滚，路径遍历防护
- 一键安装（Linux / macOS / Windows），见[安装](#安装)

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

构建输入 = docs + themes + config + Plainship 版本，因此构建可复现。

## Portability

- **内容可迁移**：不依赖 Plainship 的私有数据格式；换用任何 Markdown 工具链都可行
- **产物可迁移**：静态 HTML 可以部署到 GitHub Pages、Nginx、任意对象存储或静态托管
- **服务器可替换**：即使 Plainship Server 消失，已发布的静态站点不受影响

## Themes

主题是一个目录：`theme.json` + `layouts/` + `assets/`。

- `plainship new` 会生成 `themes/default`（也可删除它使用内嵌默认主题）
- 模板支持 `url`（根相对链接）、`t`（语言文案）、`formatDate`（语言感知日期）等函数
- 主题进 Git，是构建输入之一，变更会自动触发重建

## CLI

| 命令                        | 说明                                                                                                     |
| --------------------------- | -------------------------------------------------------------------------------------------------------- |
| `plainship new <路径>`      | 创建新的 Space（默认初始化 Git）                                                                         |
| `plainship create <名称>`   | 创建 Markdown 文档（自动补 `.md`，支持嵌套目录）                                                         |
| `plainship build [-m 消息]` | **构建 + 提交 + 编号**：检测变更 → 构建 → 成功后分步提交 Git → 打 `ps-N` 编号                            |
| `plainship publish`         | **发布**到服务器（只发布已提交源码构建出的 `build/` 内容）                                               |
| `plainship connect <地址>`  | 配置并验证服务器连接：`server.url` 写入 `plainship.yaml`，令牌写入 `.plainship/server.token`（不进 Git） |
| `plainship status [路径]`   | 查看 Git / 变更 / 构建 / 发布状态                                                                        |
| `plainship dev [--addr]`    | 本地开发模式：监听变更，自动重建并热更新浏览器（默认 `:8080`）                                           |
| `plainship serve`           | 启动 Plainship Server（`--addr` `--data` `--token`；不带 `--token` 时自动生成）                          |
| `plainship token`           | 显示服务器访问令牌（`--data` 指定数据目录）                                                              |
| `plainship version`         | 版本信息                                                                                                 |

## Quick Start

```bash
# 1. 创建 Space（自动初始化 Git）
plainship new mydoc
cd mydoc

# 2. 创建第一篇文档（自动补 .md，支持中文文件名与嵌套目录）
plainship create "测试文档"

# 3. 用你喜欢的编辑器写作
vim docs/测试文档.md

# 4. 本地实时预览（监听 docs/ themes/ 与配置变更，SSE 热更新）
plainship dev

# 5. 构建：增量构建 + 分步提交 Git + 打构建编号（失败则什么都不提交）
plainship build -m "新增测试文档"

# 6. 发布到服务器（先 plainship connect 配置连接，见「部署」）
plainship publish
```

构建结果位于 `build/`，是一个完全独立的静态网站，也可以直接静态部署：

```bash
cd build
python -m http.server 8000
# 或使用任意静态服务器 / Nginx
```

## 配置

配置文件位于 Space 根目录：`plainship.yaml`

```yaml
site:
  title: 我的文档
  description: Plainship 文档
  url: https://example.com
  language: en # 默认 en (英文); 中文站点改为 zh-CN
  siteId: my-docs

build:
  output: build

theme:
  name: default

list:
  sort: date-desc

server:
  url: http://localhost:9090 # 本地测试填 localhost，部署时改为实际服务器地址
  site: my-docs # 站点 ID，与 site.siteId 保持一致
  # token 不写入此文件（避免随 config 类别提交进 Git 历史）。
  # 用 plainship connect 写入 .plainship/server.token，或通过环境变量 PLAINSHIP_TOKEN 提供。

markdown:
  unsafe: false # 默认 false：正文中的原始 HTML 会被剔除（防 XSS）；true 时原样输出
```

### 连接服务器

推荐用 `plainship connect <服务器地址>` 配置连接：交互粘贴服务器打印的访问令牌后，`server.url` 写入 `plainship.yaml`，令牌写入 `.plainship/server.token`（0600，不进 Git）并验证令牌有效性。`publish` 未配置 `server.url` 时会直接报错，不会静默跳过。

### 多语言

Plainship 支持两级语言，互不影响：

| 级别     | 控制方式                               | 作用范围                                         |
| -------- | -------------------------------------- | ------------------------------------------------ | ------------------ |
| 工具语言 | `PLAINSHIP_LANG` 环境变量或 `--lang zh | en` 参数                                         | CLI 输出与错误提示 |
| 站点语言 | `plainship.yaml` 的 `site.language`    | 生成的网站文案（默认主题的标题 / 作者 / 日期等） |

```bash
plainship status               # 默认英文输出
plainship --lang zh status     # 中文 CLI 输出
PLAINSHIP_LANG=zh plainship build
```

Plainship **英文优先**：默认工具语言为英文，站点语言默认 `en`；中文通过 `PLAINSHIP_LANG=zh` / `--lang zh`（CLI）或 `site.language: zh-CN`（站点）切换。

### 文档 Front Matter

```yaml
---
title: 我的第一篇文章
author: Eman
date: 2026-08-13
tag: Plainship
slug: hello-world # 可选，覆盖 URL（不设置时使用文件名）
layout: article # article / page / home / list
draft: false # true 时不发布
---
```

### Markdown 支持

GFM（GitHub Flavored Markdown）：标题、段落、粗体、斜体、链接（自动解析为根相对 URL）、图片、列表、引用、代码块、表格、任务列表。正文中的原始 HTML 默认被剔除（防止发布站点 XSS）；需要直通时在 `plainship.yaml` 中设置 `markdown.unsafe: true`。

## 安装

### 客户端（CLI）

- 从 [GitHub Releases](https://github.com/emanyzwww/Plainship/releases) 下载对应平台的二进制（Linux / macOS / Windows 的 amd64 与 arm64）
- 或使用 Go 安装：`go install github.com/emanyzwww/Plainship/cmd/plainship@latest`
- 或从源码构建（见[开发](#开发)）

### 服务器

在服务器上执行一条命令，Plainship 会自动探测平台、下载最新版、启动服务并生成访问令牌：

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/emanyzwww/Plainship/master/scripts/install.sh | bash

# Windows（PowerShell）
Invoke-WebRequest -UseBasicParsing https://raw.githubusercontent.com/emanyzwww/Plainship/master/scripts/install.ps1 -OutFile install.ps1
.\install.ps1
```

脚本会完成：探测 OS / 架构 → 从 GitHub Releases 下载匹配平台的二进制并校验 SHA-256（校验失败会中止安装）→ 安装到 `/usr/local/bin`（无权限时 `~/.local/bin`；Windows 为 `%LOCALAPPDATA%\Plainship\`）→ 生成访问令牌 → 启动服务（有 systemd 时注册为服务并开机自启，否则后台运行）。

安装脚本支持固定版本与自定义参数（可复现安装，避免 "latest" 漂移）：

```bash
# 安装指定版本（推荐生产环境固定版本）
curl -fsSL https://raw.githubusercontent.com/emanyzwww/Plainship/master/scripts/install.sh | bash -s -- --version <release-tag>

# 自定义监听地址与数据目录（sh 版本还支持 --repo / --bin-dir / --no-verify 及 PS_* 环境变量）
curl -fsSL https://raw.githubusercontent.com/emanyzwww/Plainship/master/scripts/install.sh | bash -s -- --addr :9090 --data /opt/plainship/data
```

> `--no-verify` 会跳过 SHA-256 校验（不推荐）；脚本默认在任何一步失败时中止，不会留下残缺安装。

## 部署

### 一键部署（服务器端）

服务器安装脚本完成：探测 OS / 架构 → 下载匹配平台的二进制并校验 SHA-256 → 安装 → 生成访问令牌 → 启动服务（systemd 或后台运行）。命令见[安装](#服务器)。

启动完成后会打印服务器地址与访问令牌，例如：

```text
===========================================================
 Plainship vX.Y.Z 已就绪

  服务器地址: http://192.168.1.10:9090
  数据目录:   /opt/plainship/data

  访问令牌 (请复制):
  ps_3f9a2c8d4e6b1f0a2c3d4e5f6a7b8c9d

  在客户端 (Space 目录) 运行:
    plainship connect http://192.168.1.10:9090
    然后粘贴上面的令牌, 即可 plainship publish 发布
===========================================================
```

### 手动启动服务器

`plainship serve` 与安装脚本效果一致（不带 `--token` 时自动生成）：

```bash
plainship serve --addr :9090 --data ./data
# 启动信息中会醒目打印访问令牌；令牌保存在 data/server.token，重启不变
# 忘记令牌？运行：plainship token --data ./data
```

> 认证永远开启：服务器不存在「无认证」状态，客户端 `publish` 必须携带令牌。

### 客户端连接与发布

```bash
# 在 Space 目录配置并验证服务器连接（粘贴服务器打印的访问令牌）
plainship connect http://192.168.1.10:9090

# 发布（只发布由已提交源码构建出的内容）
plainship publish
```

> 令牌写入 `.plainship/server.token`（0600，不进 Git），也可用环境变量 `PLAINSHIP_TOKEN` 提供。

### 静态托管

`build/` 本身可以脱离 Plainship Server 独立部署（Nginx / GitHub Pages / 任意静态托管），站点内容与服务器完全解耦。

### 同步协议与安全

服务器只做三件事：**存储 + 同步 + 静态 HTTP**。无数据库、无 Node.js、无构建依赖；文件系统存储、原子发布（上传 → 校验 → 激活 `current` 指针，绝无半发布状态）、路径遍历防护、Token 认证（Bearer）、构建版本可回滚。完整协议与 API 见[服务器与同步协议](docs/server-and-sync.md)。

## 目录结构

```text
mydoc/
├── docs/           # 你的文档（最重要，进 Git）
├── themes/         # 主题（进 Git，构建输入之一）
├── build/          # 构建产物（不进 Git，可由 docs + themes + config 重新生成）
├── plainship.yaml  # 配置（进 Git）
└── .plainship/     # Plainship 内部状态（可全部重新生成，不进 Git）
    ├── state/      # 构建状态
    ├── cache/      # 纯缓存
    ├── manifests/  # 构建清单
    ├── builds/     # 原子构建产物（增量构建复用）
    └── server.token  # 服务器访问令牌（0600，connect 写入，不进入 Git）
```

`plainship new` 生成的 `.gitignore` 默认忽略 `.plainship/` 与 `build/`。

## Plainship 不是什么

Plainship 刻意**不包含**：

- 数据库 / CMS Runtime / 服务器端渲染
- 在线编辑器 / 评论系统 / 用户系统 / 权限系统
- Analytics / 插件市场 / SaaS / Billing
- AI 写作 / Realtime Collaboration

内容与历史属于 Git，运行时属于静态文件，Plainship 只负责构建与发布这条边界。

## Design Principles

| 关注点                             | 归属                                 |
| ---------------------------------- | ------------------------------------ |
| 源文件历史、变更、协作             | **Git**（Plainship 不重复实现）      |
| 构建缓存、映射、manifest、同步状态 | **Plainship State**（`.plainship/`） |
| 解析、渲染、构建                   | **Plainship Core**（客户端）         |
| 存储、同步、静态 HTTP              | **Plainship Server**                 |

- 英文优先：工具语言默认英文（`PLAINSHIP_LANG` / `--lang` 可切换中文），站点语言由 `site.language` 控制（默认 `en`，可设为 `zh-CN`）
- 服务器永远不做 Markdown 编译 / SSR / 数据库
- 所有复杂计算（Markdown、主题、SEO）都在客户端完成
- 保持项目小：不做 CMS、不做「大而全」

## Roadmap

- [ ] `plainship rollback <编号>` 回滚命令（内部：切换 tag 源码 + 重建）
- [x] `plainship dev` 开发模式（watch + live reload，SSE 热更新）
- [ ] `plainship dev` 增强：增量预热 / 错误覆盖层 / 自动打开浏览器
- [ ] 主题与布局系统增强（自定义组件、shortcode）
- [ ] 搜索索引 `search.json`（浏览器本地搜索）
- [ ] RSS、服务器 rollback API、基于 Host 的多站点路由

## Project Status

核心链路（创建 → 写作 → 构建 → 发布 → 静态访问 → Git 回滚）已可用，见[快速开始](#quick-start)。协议与 CLI 仍在演进，请以实际命令与文档为准。

## 文档

- [构建、提交与编号](docs/build-and-revision.md)：build 完整流程、机器提交协议、历史回滚、链接与基础路径
- [服务器与同步协议](docs/server-and-sync.md)：同步协议 JSON、API、数据布局、认证与安全
- [架构与开发](docs/architecture.md)：模块结构、依赖方向、设计原则、路线图

## 开发

要求 Go 1.26+。

```bash
go build -o plainship ./cmd/plainship   # 构建
go test ./...                            # 运行全部测试
```

CLI 只做参数解析与展示，业务逻辑全部在 Core，未来可复用给 GUI / IDE 插件 / HTTP API。模块结构、依赖方向与设计原则见[架构与开发](docs/architecture.md)。

## 贡献

欢迎通过 [Issues](https://github.com/emanyzwww/Plainship/issues) 报告问题、提出建议，或提交 Pull Request：

1. Fork 本仓库并创建功能分支
2. 修改后运行 `go test ./...` 确保测试通过
3. 提交 PR，说明改动动机与影响

建议：涉及协议或行为变更时，同步更新 [docs/](docs/) 中的对应文档；中文交流优先。

## License

[MIT](LICENSE)
