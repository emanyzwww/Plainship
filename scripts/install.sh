#!/usr/bin/env bash
#
# Plainship 一键安装脚本 (Linux / macOS)
#
# 用法:
#   curl -fsSL https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.sh | bash
#   curl -fsSL <上面的地址> | bash -s -- --addr :9090 --data /opt/plainship/data
#
# 行为:
#   1. 探测操作系统与 CPU 架构
#   2. 从 GitHub Releases 获取最新版本 (或 --version 指定版本)
#   3. 下载匹配当前平台的二进制并校验 SHA-256 (可用 --no-verify 显式跳过)
#   4. 安装到 /usr/local/bin (无权限时 ~/.local/bin)
#   5. 生成访问令牌, 启动服务 (有 systemd 时注册为服务, 否则 nohup 后台)
#   6. 打印服务器地址与访问令牌
#
set -euo pipefail

# ---- 可配置项 ----
REPO="${PS_REPO:-emanyzwww/plainship}"           # GitHub 仓库 owner/name
VERSION="${PS_VERSION:-latest}"                  # latest 或 v0.x.y 形式
ADDR="${PS_ADDR:-:9090}"                         # 监听地址
DATA_DIR="${PS_DATA:-}"                          # 数据目录 (默认按 root 判断)
BIN_DIR="${PS_BIN:-}"                            # 安装目录 (默认 /usr/local/bin)
SKIP_VERIFY="${PS_SKIP_VERIFY:-0}"               # 1 时跳过 SHA-256 校验

# ---- 颜色 (非 TTY 时自动禁用) ----
if [ -t 1 ]; then
  C_GREEN="$(printf '\033[32m')"; C_YELLOW="$(printf '\033[33m')"
  C_BOLD="$(printf '\033[1m')"; C_RESET="$(printf '\033[0m')"; C_RED="$(printf '\033[31m')"
else
  C_GREEN=''; C_YELLOW=''; C_BOLD=''; C_RESET=''; C_RED=''
fi

log()  { printf '%s\n' "$*" >&2; }
ok()   { printf '%s✓ %s%s\n' "$C_GREEN" "$*" "$C_RESET" >&2; }
warn() { printf '%s! %s%s\n' "$C_YELLOW" "$*" "$C_RESET" >&2; }
fail() { printf '%s✗ %s%s\n' "$C_RED" "$*" "$C_RESET" >&2; exit 1; }

# 依赖检查.
command -v curl >/dev/null 2>&1 || fail "curl not found; install curl and retry"

# ---- 参数解析 ----
usage() {
  cat <<EOF
Plainship one-command installer

Usage:
  install.sh [options]

Options:
  --addr <addr>       listen address (default :9090)
  --data <dir>        data dir (root: /opt/plainship/data, else ~/.plainship/data)
  --repo <owner/repo> GitHub repo (default $REPO)
  --version <ver>     install a specific version (default latest)
  --bin-dir <dir>     binary install dir (default /usr/local/bin)
  --no-verify         skip SHA-256 verification (not recommended)
  -h, --help          show this help
EOF
  exit 0
}

# 选项值校验: $1 选项名, $2 值; 缺失时报错, 避免静默吞掉.
take_value() {
  if [ $# -lt 2 ] || [ -z "$2" ]; then
    fail "option $1 requires a value"
  fi
}

while [ $# -gt 0 ]; do
  case "$1" in
    --addr)      take_value "$@"; ADDR="$2"; shift 2 ;;
    --data)      take_value "$@"; DATA_DIR="$2"; shift 2 ;;
    --repo)      take_value "$@"; REPO="$2"; shift 2 ;;
    --version)   take_value "$@"; VERSION="$2"; shift 2 ;;
    --bin-dir)   take_value "$@"; BIN_DIR="$2"; shift 2 ;;
    --no-verify) SKIP_VERIFY=1; shift ;;
    -h|--help)   usage ;;
    *)           fail "unknown option: $1 (use --help)" ;;
  esac
done

# ---- 1. 探测平台 ----
OS_NAME="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH_RAW="$(uname -m | tr '[:upper:]' '[:lower:]')"

case "$OS_NAME" in
  linux)  GOOS="linux" ;;
  darwin) GOOS="darwin" ;;
  *)      fail "unsupported OS: $OS_NAME (only linux / darwin)" ;;
esac

case "$ARCH_RAW" in
  x86_64|amd64)  GOARCH="amd64" ;;
  aarch64|arm64) GOARCH="arm64" ;;
  *) fail "unsupported architecture: $ARCH_RAW (only amd64 / arm64)" ;;
esac

BIN_NAME="plainship-$GOOS-$GOARCH"

# ---- 2. 解析版本与下载地址 ----
if [ "$VERSION" = "latest" ]; then
  API_URL="https://api.github.com/repos/$REPO/releases/latest"
else
  API_URL="https://api.github.com/repos/$REPO/releases/tags/${VERSION#v}"
  VERSION="${VERSION#v}"
fi

log "querying release info: $API_URL"
RELEASE_JSON="$(curl -fsSL --max-time 30 "$API_URL")" || fail "failed to fetch release info (repo: $REPO, version: $VERSION). Check the network or the --repo/--version options."

# 管道解析失败会触发 set -e 静默退出, 这里用 || true 保护, 由下方 fail 给出友好提示.
TAG_NAME="$(printf '%s' "$RELEASE_JSON" | grep -m1 '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')" || true
VER="${TAG_NAME#v}"
[ -n "$VER" ] || fail "cannot resolve version (repo: $REPO, version: $VERSION). Make sure the version exists and the GitHub API is not rate-limited."

log "latest version: v$VER"

# 从 assets 中按文件名找下载地址 (GitHub API 的 asset 对象中 name 在 browser_download_url 之前).
asset_url() {
  local json="$1" name="$2"
  local line
  line="$(printf '%s' "$json" | grep -n "\"name\": \"$name\"" | head -n1 | cut -d: -f1)"
  [ -n "$line" ] || return 1
  printf '%s' "$json" | tail -n "+$line" | grep -m1 '"browser_download_url"' | sed 's/.*"browser_download_url": *"\([^"]*\)".*/\1/'
}

BIN_URL="$(asset_url "$RELEASE_JSON" "$BIN_NAME")" || true
[ -n "$BIN_URL" ] || fail "repo $REPO has no $BIN_NAME asset for v$VER (make sure the Release was built for this platform)"
SHA_URL="$BIN_URL.sha256"

# ---- 3. 下载与校验 ----
TMP_DIR="$(mktemp -d)" || fail "failed to create a temporary directory"
trap 'rm -rf "$TMP_DIR"' EXIT

log "downloading: $BIN_URL"
curl -fsSL --max-time 120 -o "$TMP_DIR/$BIN_NAME" "$BIN_URL" || fail "binary download failed: $BIN_URL"

if [ "$SKIP_VERIFY" = "1" ]; then
  warn "SHA-256 verification skipped (--no-verify). Make sure the download source is trusted."
else
  curl -fsSL --max-time 30 -o "$TMP_DIR/$BIN_NAME.sha256" "$SHA_URL" \
    || fail "checksum download failed; install aborted (cannot verify binary integrity)"
  if [ -s "$TMP_DIR/$BIN_NAME.sha256" ]; then
    (cd "$TMP_DIR" && (sha256sum -c "$BIN_NAME.sha256" 2>/dev/null || shasum -a 256 -c "$BIN_NAME.sha256")) \
      || fail "SHA-256 verification failed; install aborted (tampered or incomplete download)"
    ok "SHA-256 verified"
  else
    fail "checksum file is empty; install aborted"
  fi
fi

chmod +x "$TMP_DIR/$BIN_NAME"
"$TMP_DIR/$BIN_NAME" version || fail "downloaded binary cannot run; install aborted"

# ---- 4. 安装 ----
if [ -z "$BIN_DIR" ]; then
  if [ "$(id -u)" -eq 0 ] && [ -d /usr/local/bin ]; then
    BIN_DIR=/usr/local/bin
  else
    BIN_DIR="$HOME/.local/bin"
    mkdir -p "$BIN_DIR"
  fi
fi
mkdir -p "$BIN_DIR"

if [ -x "$BIN_DIR/plainship" ]; then
  warn "old version installed; upgrading..."
fi
# 原子替换: 先复制到临时名再 mv (同文件系统内 mv 是原子的), 避免中断留下残缺文件.
cp "$TMP_DIR/$BIN_NAME" "$BIN_DIR/.plainship.new" || fail "failed to copy to $BIN_DIR"
mv -f "$BIN_DIR/.plainship.new" "$BIN_DIR/plainship"
chmod +x "$BIN_DIR/plainship"
ok "Plainship v$VER ($GOOS/$GOARCH) installed to $BIN_DIR/plainship"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) warn "$BIN_DIR is not in PATH; add the following to your shell config:"; warn "  export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac

# ---- 5. 数据目录与访问令牌 ----
if [ -z "$DATA_DIR" ]; then
  if [ "$(id -u)" -eq 0 ]; then
    DATA_DIR=/opt/plainship/data
  else
    DATA_DIR="$HOME/.plainship/data"
  fi
fi
mkdir -p "$DATA_DIR"

TOKEN_FILE="$DATA_DIR/server.token"
if [ -s "$TOKEN_FILE" ]; then
  TOKEN="$(cat "$TOKEN_FILE")"
  log "reusing existing access token"
else
  # 生成新的访问令牌 (与 plainship serve 的自动生成格式一致: ps_ + 32 hex).
  if command -v openssl >/dev/null 2>&1; then
    TOKEN="ps_$(openssl rand -hex 16)"
  else
    TOKEN="ps_$(head -c16 /dev/urandom | od -An -tx1 | tr -d '[:space:]')"
  fi
  printf '%s\n' "$TOKEN" > "$TOKEN_FILE"
  chmod 600 "$TOKEN_FILE"
  log "generated a new access token"
fi

# ---- 6. 启动服务 ----
if [ "$GOOS" = "linux" ] && [ -d /run/systemd/system ] && command -v systemctl >/dev/null 2>&1; then
  # systemd 服务 (需要 root).
  if [ "$(id -u)" -ne 0 ]; then
    warn "systemd detected, but current user is not root; cannot register a system service; falling back to nohup background start"
  else
    UNIT=/etc/systemd/system/plainship.service
    cat > "$UNIT" <<EOF
[Unit]
Description=Plainship Server
After=network.target

[Service]
Type=simple
ExecStart="$BIN_DIR/plainship" serve --addr "$ADDR" --data "$DATA_DIR"
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable plainship.service >/dev/null 2>&1 || true
    systemctl restart plainship.service || fail "systemd service failed to start; run: systemctl status plainship"
    ok "systemd service started: systemctl status plainship"
    SERVER_READY=1
  fi
fi

if [ "${SERVER_READY:-0}" != "1" ]; then
  # nohup 后台启动 (macOS / 无 systemd / 非 root).
  LOG_FILE="$DATA_DIR/plainship.log"
  PID_FILE="$DATA_DIR/plainship.pid"
  if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE" 2>/dev/null)" 2>/dev/null; then
    kill "$(cat "$PID_FILE")" 2>/dev/null || true
    sleep 1
  fi
  nohup "$BIN_DIR/plainship" serve --addr "$ADDR" --data "$DATA_DIR" >> "$LOG_FILE" 2>&1 &
  PID=$!
  echo "$PID" > "$PID_FILE"
  sleep 1
  # 启动存活检查: 立即退出说明端口占用或参数错误, 给出日志提示.
  if ! kill -0 "$PID" 2>/dev/null; then
    fail "server failed to start; see the log: $LOG_FILE"
  fi
  ok "server started in background (PID $PID, log: $LOG_FILE)"
fi

# ---- 7. 输出结果 ----
# 优先使用用户显式指定的监听地址; 仅监听全部接口 (空 host / 0.0.0.0) 时探测本机 IP.
ADDR_HOST="${ADDR%:*}"
case "$ADDR_HOST" in
  ""|"0.0.0.0"|"::"|"[::]") HOST="" ;;
  *) HOST="$ADDR_HOST" ;;
esac
if [ -z "$HOST" ]; then
  # 1) Linux: hostname -I 取第一个非回环 IPv4.
  HOST="$(hostname -I 2>/dev/null | awk '{for (i = 1; i <= NF; i++) if ($i !~ /^127\./ && $i !~ /^::1$/) { print $i; exit }}')" || true
  # 2) macOS: 默认接口地址.
  if [ -z "$HOST" ]; then
    HOST="$(ipconfig getifaddr en0 2>/dev/null)" || true
  fi
  # 3) Linux 兜底: 默认路由接口地址.
  if [ -z "$HOST" ]; then
    HOST="$(ip route get 1.1.1.1 2>/dev/null | awk '{print $7; exit}')" || true
  fi
  [ -n "$HOST" ] || HOST="localhost"
fi
PORT="${ADDR##*:}"

cat <<EOF

$C_BOLD===========================================================$C_RESET
$C_BOLD Plainship v$VER ready$C_RESET

  Server URL: http://$HOST:$PORT
  Data dir:   $DATA_DIR

  Access token (copy this):
  $C_GREEN$TOKEN$C_RESET

  On your client (in the Space dir) run:
    plainship connect http://$HOST:$PORT
    then paste the token, and run plainship publish

  Forgot the token? On the server run: plainship token --data $DATA_DIR
$C_BOLD===========================================================$C_RESET

EOF
