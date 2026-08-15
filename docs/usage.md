# 使用指南

> 本文是 Plainship 的完整使用参考:CLI 命令,配置,Front matter,多语言,目录结构与回滚.
> 快速上手见 [README](../README.md).

## CLI

### 客户端 `plainship`

| 命令                        | 说明                                                                               |
| --------------------------- | ---------------------------------------------------------------------------------- |
| `plainship new <路径>`      | 创建新的 Space(默认初始化 Git)                                                     |
| `plainship create <名称>`   | 创建 Markdown 文档(自动补 `.md`,支持嵌套目录与中文文件名)                          |
| `plainship build [-m 消息]` | 构建 + 分步提交 Git + 打 `ps-N` 编号(失败则不提交任何东西)                         |
| `plainship publish [-y]`    | 发布到服务器(只发布已提交源码构建出的内容,发布前四重校验;`-y` 跳过确认)            |
| `plainship connect <地址> [--token <令牌>]` | 配置并验证服务器连接(url 写入 `plainship.yaml`,令牌写入 `.plainship/config.yaml`) |
| `plainship status [路径]`   | 查看 Git / 变更 / 构建 / 发布状态                                                  |
| `plainship dev [--addr]`    | 本地开发模式:监听变更自动重建,SSE 热更新浏览器(默认 `:8080`)                       |
| `plainship preview [--addr] [--open]` | 发布前在本地预览已构建的站点(`--open` 自动打开浏览器)                     |
| `plainship config get|set|unset <键> [-g]` | 配置 CLI 行为(get/set/unset 子命令,当前仅支持 `lang`;`-g` 读写全局配置)   |
| `plainship version`         | 显示版本信息                                                                       |

客户端全局标志(所有命令可用):

| 标志            | 说明                                                     |
| --------------- | -------------------------------------------------------- |
| `--lang zh|en` | 界面语言(默认取 `PLAINSHIP_LANG` 或配置)               |
| `--no-color`   | 禁用彩色输出(等价于 `NO_COLOR` 环境变量)               |
| `--json`       | 以 JSON 事件流输出(机器可读,供脚本 / CI 消费)            |
| `--verbose` / `-v` | 输出调试日志到 stderr(slog)                        |

### 服务端 `plainship-server`

| 命令                       | 说明                                                                      |
| -------------------------- | ------------------------------------------------------------------------- |
| `plainship-server serve`   | 启动服务器(`--addr` `--data` `--token` `--log-level` `--log-file` `--log-format`;不带 `--token` 时自动生成并持久化) |
| `plainship-server token`   | 显示服务器访问令牌(`--data` 指定数据目录)                                 |
| `plainship-server version` | 显示版本信息(与客户端共享版本号)                                          |

服务端运行日志(serve 命令标志):

| 标志                | 说明                                            |
| ------------------- | ----------------------------------------------- |
| `--log-level`     | 日志级别:debug / info / warn / error(默认 info) |
| `--log-file`      | 日志文件路径(默认 stderr)                       |
| `--log-format`    | 日志格式:text / json(默认 text)                 |

运行日志记录 HTTP 访问(method / path / status / duration)、同步成功与失败、激活与鉴权事件;用户横幅(地址 / token / API 列表)输出到 stdout.

## 配置

配置文件位于 Space 根目录:`plainship.yaml`

```yaml
site:
  title: 我的文档
  description: Plainship 文档
  url: https://example.com
  language: en # 默认 en;中文站点改为 zh-CN
  siteId: my-docs

build:
  output: build

theme:
  name: default

list:
  sort: date-desc

server:
  url: http://localhost:9090
  site: my-docs # 站点 ID,与 site.siteId 一致
  # 令牌不写入此文件(避免进 Git 历史),用 plainship connect 写入 .plainship/config.yaml

markdown:
  unsafe: false # 默认 false:正文中的原始 HTML 会被转义(防 XSS);true 时原样输出
```

## Front matter

```yaml
---
title: 我的第一篇文章
author: Eman
date: 2026-08-13
tag: Plainship
slug: hello-world # 可选,覆盖 URL(不设置时使用文件名)
layout: article # article / page / home / list
draft: false # true 时不发布
---
```

## Markdown 支持

GFM(GitHub Flavored Markdown):标题,段落,粗体,斜体,链接(自动解析为根相对 URL),图片,列表,引用,代码块(语法高亮),表格,任务列表.正文中的原始 HTML 默认被转义(防发布站点 XSS);需要直通时设置 `markdown.unsafe: true`.

## 多语言

Plainship 支持两级语言,互不影响:

| 级别     | 控制方式                              | 作用范围           |
| -------- | ------------------------------------- | ------------------ |
| 工具语言 | `PLAINSHIP_LANG` 环境变量或 `--lang zh|en` | CLI 输出与错误提示 |
| 站点语言 | `plainship.yaml` 的 `site.language`    | 生成网站的主题文案(默认 `en`,中文站点改 `zh-CN`) |

Plainship 英文优先:默认工具语言为英文,可通过 `PLAINSHIP_LANG=zh` / `--lang zh` 切换中文.

## 目录结构

```text
mydoc/
├── docs/           # 你的文档(最重要,进 Git)
├── themes/         # 主题(进 Git,构建输入之一)
├── build/          # 构建产物(不进 Git,可由源码重新生成)
├── plainship.yaml  # 配置(进 Git)
└── .plainship/     # 内部状态(除 config.yaml 中的令牌外均可重新生成,不进 Git)
    ├── state/      # 构建状态
    ├── cache/      # 纯缓存
    ├── manifests/  # 构建清单
    ├── builds/     # 原子构建产物(增量构建复用)
    └── config.yaml # 客户端 CLI 空间配置,含服务器访问令牌(0600,connect 写入,不进入 Git)
```

`plainship new` 生成的 `.gitignore` 默认忽略 `.plainship/` 与 `build/`.

## 主题

主题是一个目录:`theme.json` + `layouts/` + `assets/`.`plainship new` 会生成 `themes/default`(也可删除它使用内嵌默认主题).模板提供 `url`(根相对链接),`t`(语言文案),`formatDate`(语言感知日期)等函数.主题进 Git,是构建输入之一.

## 本地开发

`plainship dev` 监听 `docs/`,`themes/` 与 `plainship.yaml` 的变更,自动重建并通过 SSE 热更新浏览器(默认 `:8080`).dev 模式只构建,不提交 Git,不打编号.

## 静态产物

`build/` 是完全独立的静态网站,可脱离 Plainship Server 部署:

```bash
cd build
python -m http.server 8000   # 或任意静态服务器 / Nginx / GitHub Pages
```

站点内所有链接都是根相对地址,任意页面深度与任意部署方式下都正确.

## 回滚

回滚的本质是"还原源码 + 重新构建":

```bash
git checkout ps-0003     # 还原到某个 Release 的源码
plainship build
plainship publish        # 重新发布
```

- 单篇回滚:`git log -- docs/某篇.md` 找到对应编号 → `git checkout ps-0003 -- docs/某篇.md` → `plainship build`
- 整站回滚:`git checkout ps-0003` → `plainship build` → `plainship publish`
- 服务器保留全部 release 快照;服务器端 rollback API 在路线图中