<p align="center">
  <img src="assets/hero.png" alt="Plainship" width="720">
</p>

<p align="center">
  <strong>Ship your Git-native content as durable static websites.</strong>
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

## What is Plainship

Plainship is a local-first, Git-first publishing system for Markdown content.

- Content lives in a Git repository as plain Markdown; Git owns history, diffs, and collaboration.
- One command incrementally builds a static website, commits the result, and assigns a build number.
- One command publishes the site to your own server; the server only stores, syncs, and serves static HTTP.
- Two small static binaries — `plainship` and `plainship-server` — are released together with the same version.

## Quick start

### On the server

One command installs `plainship-server`, starts the service, and prints the server URL and access token.

**Linux / macOS**

```bash
# Install and start the server, then print the URL and access token
curl -fsSL https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.sh | bash
```

**Windows (PowerShell)**

```powershell
# Download the installer
Invoke-WebRequest -UseBasicParsing https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.ps1 -OutFile install.ps1
# Install and start the server
.\install.ps1
```

### On your computer

Install the latest client:

```bash
# Install the client
go install github.com/emanyzwww/plainship/cmd/plainship@latest
```

Create a new project and publish your first document:

```bash
# Create a new project (initializes Git)
plainship new mydoc
cd mydoc

# Create a document
plainship create "my-first-doc"
# Build: commit and assign a build number
plainship build -m "first document"

# Connect to your server (paste the token when prompted)
plainship connect http://<server>:9090

# Publish to the server
plainship publish
```

The generated `build/` directory is a self-contained static site that can be deployed to any static host.

For local development with hot reload:

```bash
# Local preview with hot reload
plainship dev
```

## Documentation

- [Usage guide](docs/usage.md) — CLI reference, configuration, front matter, directory layout, languages, and rollback.
- [Build & publishing](docs/publishing.md) — build, commit, and numbering workflow, releases, and design principles.
- [Build, commit & revision](docs/build-and-revision.md) — build mechanism, machine commit protocol, and base paths.
- [Server & sync protocol](docs/server-and-sync.md) — server deployment, installation, and the sync API.
- [Architecture & development](docs/architecture.md) — module structure, dependency direction, and development workflow.
- [Output & UX architecture](docs/output-architecture.md) — the internal/ui event-stream design (implemented, v2).

## Development

Plainship requires **Go 1.26+**.

Build the binaries:

```bash
# Build the client
go build -o plainship ./cmd/plainship
# Build the server
go build -o plainship-server ./cmd/plainship-server
```

Run the test suite:

```bash
# Run all tests
go test ./...
```

## Design principles

Plainship is built around a few simple principles:

- **Git-first** — Git remains the source of truth for content, history, and collaboration.
- **Local-first** — authoring and building happen locally, without requiring a hosted CMS.
- **Static by default** — the published site is static and can be served by almost any HTTP server.
- **Simple deployment** — the server is intentionally small and only handles storage, synchronization, and serving.
- **Durable content** — Markdown files and Git history remain usable without Plainship.

## License

[MIT](LICENSE)
