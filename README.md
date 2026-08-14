# Plainship

> [中文](README.zh-CN.md) | **English**

> **Plainship — ship your Git-native content as durable static websites.**

Your content already lives in Markdown + Git. Plainship solves the last step:

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

One directory + Markdown + Git + one command = a website you can publish and roll back.

![Go](https://img.shields.io/badge/go-1.26.5-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-lightgrey)

## What problem does Plainship solve?

Plainship is not just another static site generator. It is a **Publishing Layer** that turns content already living in Git into a **publishable website**:

- Maintain local Markdown files with the editor you already use (VS Code / Vim / Neovim)
- Content history, diffs, branches, collaboration and recovery all belong to Git (Plainship does not reimplement version control)
- One command incrementally builds a complete static site and publishes it as an **immutable Release**
- The server only stores, syncs and serves static HTTP: no database, no CMS runtime, no server-side build

## Core workflow

```text
Git owns:     content · history · diff · branch · collaboration · recovery
Plainship:    build → preview → publish → release → rollback → serve
Static files: runtime
```

- **Git owns the content.**
- **Plainship owns publishing.**
- **Static files own the runtime.**

## Why Plainship

- **Git-native**: content is Markdown in a Git repository, no proprietary formats
- **Local-first**: everything is built on your machine; your editor, terminal and Git workflow stay the same
- **One-command publishing**: `plainship build` incrementally builds, commits and assigns a build number; `plainship publish` syncs to the server
- **Static-first**: `build/` is a fully self-contained static site, independent of databases, CMS or server-side rendering
- **Rollback**: every build has a number (`ps-N` tag); restore the sources and rebuild to roll back
- **Portable**: if you stop using Plainship, Markdown + Git still work with any other toolchain, and the static HTML can be deployed to any host
- **Tiny**: a single static binary with no runtime dependencies

## Git is the source of truth

Plainship does not replace Git and does not create a second source of truth:

- `docs/`, `themes/` and `plainship.yaml` are all committed to Git
- `plainship build` commits in steps (config / theme / docs) with machine-readable messages
- Build numbers are recorded as Git tags (`ps-0001`, `ps-0002`, ...); history lives in Git
- `build/` and `.plainship/` stay out of Git: they are **reproducible** from sources + the Plainship version

## One command to publish

```bash
plainship publish
```

`publish` verifies first (it refuses to publish anything half-baked):

1. No uncommitted changes in the current sources (config / theme / docs all clean)
2. `build/` was built from the current sources (category fingerprints match)
3. `build/` came from a production build (prevents publishing dev output)
4. The renderer version matches the current binary (prevents publishing stale rendering after upgrades)

After verification, it uploads the diff incrementally and the server activates it atomically (upload → verify → switch the `current` pointer, never a half-published state).

## Release / Version

Every successful build is a **Release**:

- Build number: `ps-0001`, `ps-0002`, ... (a Git tag pointing at the last commit of that build)
- Machine commit protocol: `<category> build=<number> hash=<16-char fingerprint>` — parseable and verifiable
- Server side: every sync stores a **complete snapshot** under `data/sites/<siteId>/releases/<buildId>/` with build metadata (`release.json`)
- Incremental publishing: the client uploads only the diff; the server completes each release from the previous one, so every release is a full snapshot

## Rollback

Rollback is simply "restore sources + rebuild":

```bash
git checkout ps-0003     # restore the sources of a Release
plainship build
plainship publish        # republish
```

- Single document: `git log -- docs/some-doc.md` → `git checkout ps-0003 -- docs/some-doc.md` → `plainship build`
- Whole site: `git checkout ps-0003` → `plainship build` → `plainship publish`
- The server keeps every release snapshot; a server-side rollback API is on the roadmap

## Local development

```bash
plainship dev
```

Watches `docs/`, `themes/` and `plainship.yaml`, rebuilds automatically and hot-reloads the browser over SSE (default `:8080`). Dev mode only builds: no Git commits, no build numbers.

## Static output

`build/` is a fully self-contained static site that can be deployed without the Plainship Server:

```bash
cd build
python -m http.server 8000   # or any static server / Nginx / GitHub Pages
```

All in-site links are generated as **root-relative URLs**, correct at any page depth and under any deployment.

## Self-hosted server

```bash
plainship serve --addr :9090 --data ./data
```

The server does only three things: **storage + sync + static HTTP**.

- No database, no Node.js, no build dependencies
- Auth is always on (Bearer token, auto-generated and persisted to `data/server.token`)
- Atomic publishing: upload → verify → activate the `current` pointer
- Build versions can be rolled back; path traversal is guarded
- One-command installation (Linux / macOS / Windows), see [Installation](#installation)

## Artifact model

```text
sources (docs + themes + plainship.yaml)
      ↓ plainship build
static site snapshot (build/)
      ↓ plainship publish
server release (data/sites/<siteId>/releases/<buildId>/)
      ↓ activate
live version (current.json pointer → static HTTP)
```

Build inputs = docs + themes + config + Plainship version, so builds are reproducible.

## Portability

- **Content is portable**: no proprietary data formats; any Markdown toolchain works
- **Output is portable**: static HTML can be deployed to GitHub Pages, Nginx, object storage or any static host
- **The server is replaceable**: even if the Plainship Server disappears, published static sites keep working

## Themes

A theme is a directory: `theme.json` + `layouts/` + `assets/`.

- `plainship new` generates `themes/default` (you can delete it and use the embedded default theme)
- Templates provide functions like `url` (root-relative links), `t` (localized text) and `formatDate` (locale-aware dates)
- Themes are committed to Git and are build inputs; changes trigger a rebuild

## CLI

| Command                    | Description                                                                                                                                    |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `plainship new <path>`     | Create a new Space (initializes Git by default)                                                                                                |
| `plainship create <name>`  | Create a Markdown document (appends `.md`; supports nested directories)                                                                        |
| `plainship build [-m msg]` | **Build + commit + number**: detect changes → build → commit in steps → tag `ps-N`                                                             |
| `plainship publish`        | **Publish** to the server (only content built from committed sources)                                                                          |
| `plainship connect <url>`  | Configure and verify the server connection: writes `server.url` to `plainship.yaml` and the token to `.plainship/server.token` (not committed) |
| `plainship status [path]`  | Show the Space's Git / changes / build / publish status                                                                                        |
| `plainship dev [--addr]`   | Local dev mode: watch, rebuild and hot-reload the browser (default `:8080`)                                                                    |
| `plainship serve`          | Start the Plainship Server (`--addr` `--data` `--token`; auto-generates a token if omitted)                                                    |
| `plainship token`          | Show the server access token (`--data` selects the data directory)                                                                             |
| `plainship version`        | Show version information                                                                                                                       |

## Quick start

```bash
# 1. Create a Space (initializes Git automatically)
plainship new mydoc
cd mydoc

# 2. Create the first document (appends .md; Chinese file names and nested dirs work)
plainship create "my-first-doc"

# 3. Write with your favorite editor
vim docs/my-first-doc.md

# 4. Local live preview (watches docs/ themes/ and the config; SSE hot reload)
plainship dev

# 5. Build: incremental build + step-by-step Git commit + build number (nothing is committed on failure)
plainship build -m "first document"

# 6. Publish to the server (run plainship connect first, see Deployment)
plainship publish
```

The build output lives in `build/`, a fully self-contained static site that can also be deployed directly:

```bash
cd build
python -m http.server 8000
# or any static server / Nginx
```

## Configuration

The config file lives at the Space root: `plainship.yaml`

```yaml
site:
  title: My Docs
  description: Plainship docs
  url: https://example.com
  language: en # default en; use zh-CN for a Chinese site
  siteId: my-docs

build:
  output: build

theme:
  name: default

list:
  sort: date-desc

server:
  url: http://localhost:9090 # use localhost for local testing; change to the real server URL in production
  site: my-docs # site ID, must match site.siteId
  # The token is not written to this file (it would be committed into Git history).
  # Use plainship connect to write .plainship/server.token, or provide it via the PLAINSHIP_TOKEN env var.

markdown:
  unsafe: false # default false: raw HTML in content is stripped (XSS protection); true passes it through
```

### Connecting to a server

Use `plainship connect <server-url>` to configure the connection: after pasting the access token printed by the server, `server.url` is written to `plainship.yaml` and the token to `.plainship/server.token` (0600, not committed), and the token is verified. `publish` fails loudly if `server.url` is not configured — it never silently skips.

### Languages

Plainship supports two levels of language, independently:

| Level | Control                                            | Scope                                                           |
| ----- | -------------------------------------------------- | --------------------------------------------------------------- |
| Tool  | `PLAINSHIP_LANG` env var or `--lang zh\|en`        | CLI output and error messages                                   |
| Site  | `site.language` in `plainship.yaml` (default `en`) | Generated site copy (default theme title / author / date, etc.) |

```bash
plainship status               # English output by default
plainship --lang zh status     # Chinese CLI output
PLAINSHIP_LANG=zh plainship build
```

Plainship is **English-first**: the default tool language is English and the default site language is `en`. Chinese is supported via `PLAINSHIP_LANG=zh` / `--lang zh` (CLI) or `site.language: zh-CN` (site).

### Document front matter

```yaml
---
title: My first article
author: Eman
date: 2026-08-13
tag: Plainship
slug: hello-world # optional; overrides the URL (defaults to the file name)
layout: article # article / page / home / list
draft: false # true hides the document from publishing
---
```

### Markdown support

GFM (GitHub Flavored Markdown): headings, paragraphs, bold, italics, links (resolved to root-relative URLs automatically), images, lists, quotes, code blocks, tables, task lists. Raw HTML in content is stripped by default (XSS protection); set `markdown.unsafe: true` in `plainship.yaml` to pass it through.

## Installation

### Client (CLI)

- Download the binary for your platform from the [GitHub Releases](https://github.com/emanyzwww/plainship/releases) (Linux / macOS / Windows, amd64 & arm64)
- Or install with Go: `go install github.com/emanyzwww/plainship/cmd/plainship@latest`
- Or build from source (see [Development](#development))

### Server

Run one command on the server; Plainship detects the platform, downloads the latest release, starts the service and generates an access token:

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.sh | bash

# Windows (PowerShell)
Invoke-WebRequest -UseBasicParsing https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.ps1 -OutFile install.ps1
.\install.ps1
```

The script detects OS / architecture → downloads the matching binary from GitHub Releases and verifies its SHA-256 (aborts on mismatch) → installs to `/usr/local/bin` (or `~/.local/bin` without permission; `%LOCALAPPDATA%\Plainship\` on Windows) → generates an access token → starts the service (registers a systemd service with autostart when available, otherwise runs in the background).

The install scripts support pinned versions and custom parameters (reproducible installs, avoiding "latest" drift):

```bash
# Install a specific version (pin the version in production)
curl -fsSL https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.sh | bash -s -- --version <release-tag>

# Custom listen address and data dir (the sh script also supports --repo / --bin-dir / --no-verify and PS_* env vars)
curl -fsSL https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.sh | bash -s -- --addr :9090 --data /opt/plainship/data
```

> `--no-verify` skips SHA-256 verification (not recommended); the script aborts on any failure and never leaves a partial install.

## Deployment

### One-command deployment (server side)

The server install script detects OS / architecture → downloads the matching binary and verifies SHA-256 → installs → generates an access token → starts the service (systemd or background). See [Installation](#server).

When it finishes, it prints the server URL and access token, e.g.:

```text
===========================================================
 Plainship vX.Y.Z ready

  Server URL: http://192.168.1.10:9090
  Data dir:   /opt/plainship/data

  Access token (copy this):
  ps_3f9a2c8d4e6b1f0a2c3d4e5f6a7b8c9d

  On your client (in the Space dir) run:
    plainship connect http://192.168.1.10:9090
  then paste the token, and run plainship publish
===========================================================
```

### Starting the server manually

`plainship serve` behaves like the install script (auto-generates a token when `--token` is omitted):

```bash
plainship serve --addr :9090 --data ./data
# The startup output prints the access token prominently; it is saved to data/server.token and persists across restarts
# Forgot the token? Run: plainship token --data ./data
```

> Auth is always on: the server has no "no-auth" state; the client `publish` must carry a token.

### Client connection and publishing

```bash
# Configure and verify the server connection from the Space directory (paste the token printed by the server)
plainship connect http://192.168.1.10:9090

# Publish (only content built from committed sources)
plainship publish
```

> The token is written to `.plainship/server.token` (0600, not committed); the `PLAINSHIP_TOKEN` environment variable also works.

### Static hosting

`build/` can be deployed independently of the Plainship Server (Nginx / GitHub Pages / any static host); site content and the server are fully decoupled.

### Sync protocol and security

The server does only three things: **storage + sync + static HTTP**. No database, no Node.js, no build dependencies; filesystem storage, atomic publishing (upload → verify → activate the `current` pointer, never a half-published state), path-traversal guards, Bearer token auth, and rollback across build versions. See [Server & sync protocol](docs/server-and-sync.md) for the full protocol and API.

## Directory layout

```text
mydoc/
├── docs/           # your documents (the core; committed to Git)
├── themes/         # themes (committed to Git; build inputs)
├── build/          # build output (not in Git; reproducible from docs + themes + config)
├── plainship.yaml  # config (committed to Git)
└── .plainship/     # Plainship internal state (fully regenerable; not in Git)
    ├── state/      # build state
    ├── cache/      # pure cache
    ├── manifests/  # build manifests
    ├── builds/     # atomic build outputs (reused by incremental builds)
    └── server.token  # server access token (0600, written by connect, never committed)
```

The `.gitignore` generated by `plainship new` ignores `.plainship/` and `build/` by default.

## What Plainship is not

Plainship deliberately does **not** include:

- Databases / CMS runtimes / server-side rendering
- Online editors / comment systems / user systems / permission systems
- Analytics / plugin marketplaces / SaaS / billing
- AI writing / realtime collaboration

Content and history belong to Git, the runtime belongs to static files; Plainship owns only the build-and-publish boundary.

## Design principles

| Concern                                    | Owner                                       |
| ------------------------------------------ | ------------------------------------------- |
| Source history, changes, collaboration     | **Git** (Plainship does not reimplement it) |
| Build cache, mapping, manifest, sync state | **Plainship State** (`.plainship/`)         |
| Parsing, rendering, building               | **Plainship Core** (client)                 |
| Storage, sync, static HTTP                 | **Plainship Server**                        |

- English-first; the default tool language is English, switchable to Chinese via `PLAINSHIP_LANG` / `--lang`; the site language is controlled by `site.language`
- The server never compiles Markdown, does SSR or uses a database
- All heavy computation (Markdown, themes, SEO) happens on the client
- Stay small: no CMS, no "everything and the kitchen sink"

## Roadmap

- [ ] `plainship rollback <number>` command (internally: switch tag sources + rebuild)
- [x] `plainship dev` dev mode (watch + live reload via SSE)
- [ ] `plainship dev` enhancements: incremental warm-up / error overlay / auto-open browser
- [ ] Theme and layout enhancements (custom components, shortcodes)
- [ ] Search index `search.json` (in-browser local search)
- [ ] RSS, server rollback API, multi-site routing by Host

## Project status

The core pipeline (create → write → build → publish → static access → Git rollback) works, see [Quick start](#quick-start). The protocol and CLI are still evolving; follow the actual commands and docs.

## Documentation

- [Build, commit & numbering](docs/build-and-revision.md): the full build flow, machine commit protocol, history rollback, links and base path
- [Server & sync protocol](docs/server-and-sync.md): sync protocol JSON, API, data layout, auth and security
- [Architecture & development](docs/architecture.md): module structure, dependency direction, design principles, roadmap

## Development

Requires Go 1.26+.

```bash
go build -o plainship ./cmd/plainship   # build
go test ./...                            # run all tests
```

The CLI only parses arguments and renders output; all business logic lives in Core, reusable later for GUI / IDE plugins / HTTP APIs. See [Architecture & development](docs/architecture.md) for module structure, dependency direction and design principles.

## Contributing

Report issues or suggest ideas via [Issues](https://github.com/emanyzwww/plainship/issues), or open a Pull Request:

1. Fork this repository and create a feature branch
2. Run `go test ./...` to make sure tests pass
3. Open a PR explaining the motivation and impact

Please update the corresponding docs in [docs/](docs/) when changing protocol or behavior.

## License

[MIT](LICENSE)
