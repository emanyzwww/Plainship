<p align="center">
  <img src="assets/hero.png" alt="Plainship" width="720">
</p>

<p align="center">
  <strong>面向 Git 原生内容的极简发布系统.</strong>
</p>

<p align="center">
  <a href="README.md">English</a> |
  <a href="README.zh-CN.md">中文</a>
</p>

<p align="center">
  <a href="https://github.com/emanyzwww/plainship/actions/workflows/test.yml">
    <img src="https://github.com/emanyzwww/plainship/actions/workflows/test.yml/badge.svg" alt="CI">
  </a>
  <a href="https://github.com/emanyzwww/plainship/releases">
    <img src="https://img.shields.io/github/v/release/emanyzwww/plainship" alt="Release">
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/badge/license-MIT-green" alt="License">
  </a>
  <a href="https://github.com/emanyzwww/plainship/releases">
    <img src="https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-lightgrey" alt="Platform">
  </a>
</p>

## Plainship 是什么

Plainship 是一个本地优先, Git 优先的 Markdown 文档发布系统.

- 内容就是 Git 仓库里的 Markdown, 历史, 差异与协作全部交给 Git.
- 一个命令增量构建为静态网站, 提交结果并分配构建编号.
- 一个命令把站点发布到自己的服务器; 服务器只做存储, 同步与静态 HTTP.
- 两个小型静态二进制 -- `plainship` 与 `plainship-server` -- 同一版本一起发布.

## 快速开始

### 在服务器上

一条命令安装 `plainship-server`, 启动服务, 并打印服务器地址与访问令牌.

**Linux / macOS**

```bash
# 安装并启动服务器, 完成后打印服务器地址与访问令牌
curl -fsSL https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.sh | bash
```

**Windows (PowerShell)**

```powershell
# 下载安装脚本
Invoke-WebRequest -UseBasicParsing https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.ps1 -OutFile install.ps1
# 安装并启动服务器
.\install.ps1
```

### 在自己的电脑上

安装最新客户端:

```bash
# 安装客户端
go install github.com/emanyzwww/plainship/cmd/plainship@latest
```

创建项目并发布第一篇文档:

```bash
# 创建项目 (自动初始化 Git)
plainship new mydoc
cd mydoc

# 创建文档
plainship create "第一篇文档"
# 构建: 提交并分配构建编号
plainship build -m "第一篇文档"

# 连接服务器 (按提示粘贴访问令牌)
plainship connect http://<服务器地址>:9090

# 发布到服务器
plainship publish
```

生成的 `build/` 目录是完全独立的静态网站, 可部署到任意静态托管.

本地开发热更新预览:

```bash
# 本地预览 (热更新)
plainship dev
```

## 文档

- [使用指南](docs/usage.md) -- CLI 命令, 配置, Front matter, 目录结构, 多语言与回滚.
- [构建与发布](docs/publishing.md) -- 构建, 提交与编号流程, Release 与设计原则.
- [服务器与同步协议](docs/server-and-sync.md) -- 服务器部署, 安装与同步 API.
- [架构与开发](docs/architecture.md) -- 模块结构, 依赖方向与开发流程.

## 开发

要求 **Go 1.26+**.

构建二进制:

```bash
# 构建客户端
go build -o plainship ./cmd/plainship
# 构建服务端
go build -o plainship-server ./cmd/plainship-server
```

运行测试:

```bash
# 运行全部测试
go test ./...
```

## 设计原则

Plainship 围绕几个简单的原则构建:

- **Git-first** -- Git 始终是内容, 历史与协作的唯一事实来源.
- **Local-first** -- 写作与构建都在本地完成, 不依赖托管的 CMS.
- **Static by default** -- 发布出的站点是静态的, 几乎任何 HTTP 服务器都能托管.
- **Simple deployment** -- 服务器刻意保持小巧, 只处理存储, 同步与提供服务.
- **Durable content** -- 离开 Plainship 后, Markdown 文件与 Git 历史依然可用.

## License

[MIT](LICENSE)
