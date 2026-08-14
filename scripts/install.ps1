# Plainship 一键安装脚本 (Windows)
#
# 用法 (PowerShell):
#   Invoke-WebRequest -UseBasicParsing https://raw.githubusercontent.com/emanyzwww/plainship/master/scripts/install.ps1 -OutFile install.ps1
#   .install.ps1
#   .install.ps1 -Addr :9090 -DataDir C:plainship-data
#
# 行为:
#   1. 检测 CPU 架构 (amd64 / arm64)
#   2. 从 GitHub Releases 获取最新版本 (或 -Version 指定)
#   3. 下载匹配平台的 plainship-windows-<arch>.exe 并校验 SHA-256 (可 -NoVerify 跳过)
#   4. 停止旧实例后安装到 %LOCALAPPDATA%\Plainship\plainship.exe
#   5. 生成访问令牌, 后台启动服务 (日志 + PID 文件)
#   6. 打印服务器地址与访问令牌

param(
    [string]$Addr = ':9090',
    [string]$DataDir = '',
    [string]$Repo = 'emanyzwww/plainship',
    [string]$Version = 'latest',
    [switch]$NoVerify
)

$ErrorActionPreference = 'Stop'

# Windows PowerShell 5.1 默认可能协商到 TLS1.0, GitHub 要求 TLS1.2+.
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

function Write-OK   { Write-Host ("  OK  $args") -ForegroundColor Green }
function Write-Warn { Write-Host ("  !!  $args") -ForegroundColor Yellow }
function Write-Fail { Write-Host ("  !!  $args") -ForegroundColor Red; exit 1 }

# ---- 1. 探测架构 ----
$arch = $env:PROCESSOR_ARCHITECTURE
# 32 位 PowerShell (WOW64) 下取真实架构.
if (-not $arch) { $arch = $env:PROCESSOR_ARCHITEW6432 }
switch -Regex ($arch) {
    '^(AMD64|x86_64)$' { $goarch = 'amd64' }
    '^(ARM64|arm64)$'  { $goarch = 'arm64' }
    default { Write-Fail "Unsupported architecture: $arch (amd64 / arm64 only)" }
}
$binName = "plainship-windows-$goarch.exe"

# ---- 2. 版本信息 ----
if ($Version -eq 'latest') {
    $apiUrl = "https://api.github.com/repos/$Repo/releases/latest"
} else {
    $apiUrl = "https://api.github.com/repos/$Repo/releases/tags/$($Version.TrimStart('v'))"
}
Write-Host "Querying release info: $apiUrl"
try {
    $release = Invoke-RestMethod -Uri $apiUrl -Headers @{ 'User-Agent' = 'plainship-installer' } -TimeoutSec 30
} catch {
    Write-Fail "Failed to query release info: $($_.Exception.Message)"
}
$ver = $release.tag_name.TrimStart('v')
if (-not $ver) { Write-Fail 'Cannot resolve version' }
Write-Host "Latest version: v$ver"

$asset = $release.assets | Where-Object { $_.name -eq $binName } | Select-Object -First 1
if (-not $asset) { Write-Fail "Release v$ver of $Repo has no $binName asset" }
$shaAsset = $release.assets | Where-Object { $_.name -eq "$binName.sha256" } | Select-Object -First 1

# ---- 3. 下载与校验 ----
$localAppData = $env:LOCALAPPDATA
if (-not $localAppData) { $localAppData = Join-Path $env:USERPROFILE 'AppData\Local' }
$installDir = Join-Path $localAppData 'Plainship'
$exePath = Join-Path $installDir 'plainship.exe'
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

$tmp = Join-Path $installDir "$binName.download"
Write-Host "Downloading: $($asset.browser_download_url)"
Invoke-WebRequest -UseBasicParsing -Uri $asset.browser_download_url -OutFile $tmp -TimeoutSec 300

if ($NoVerify) {
    Write-Warn 'SHA-256 verification skipped (-NoVerify). Make sure the download source is trusted.'
} else {
    if (-not $shaAsset) { Remove-Item $tmp -Force -ErrorAction SilentlyContinue; Write-Fail "Checksum file not found in release; install aborted (cannot verify binary integrity)" }
    $expected = ((Invoke-WebRequest -UseBasicParsing -Uri $shaAsset.browser_download_url -TimeoutSec 30).Content -split '\s+')[0]
    if (-not $expected) { Remove-Item $tmp -Force -ErrorAction SilentlyContinue; Write-Fail "Checksum file is empty; install aborted" }
    $actual = (Get-FileHash -Algorithm SHA256 -Path $tmp).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { Remove-Item $tmp -Force; Write-Fail "SHA-256 mismatch (expected $expected, got $actual)" }
    Write-OK 'SHA-256 verified'
}

# ---- 4. 数据目录 (提前确定, 停旧进程需要 PID 文件路径) ----
if (-not $DataDir) { $DataDir = Join-Path $installDir 'data' }
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
$pidFile = Join-Path $DataDir 'plainship.pid'

# ---- 5. 停止旧实例并替换二进制 ----
# 必须先停后换: 运行中的 exe 被占用, Move-Item 会失败导致升级中断.
function Stop-RunningPlainship {
    param([string]$PidFile)
    # 按进程名停止 (不依赖 PID 文件, 覆盖手动启动的场景).
    Get-Process -Name 'plainship' -ErrorAction SilentlyContinue | ForEach-Object {
        Write-Host "  Stopping existing plainship (PID $($_.Id))"
        Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
    }
    # PID 文件兜底: 仅当其中 PID 确实指向 plainship 进程时才停止, 防止误杀.
    if (Test-Path $PidFile) {
        $oldPid = Get-Content $PidFile -ErrorAction SilentlyContinue
        if ($oldPid) {
            $p = Get-Process -Id $oldPid -ErrorAction SilentlyContinue
            if ($p -and $p.ProcessName -eq 'plainship') {
                Stop-Process -Id $oldPid -Force -ErrorAction SilentlyContinue
            }
        }
    }
    Start-Sleep -Milliseconds 500
}

Stop-RunningPlainship -PidFile $pidFile
Move-Item -Force $tmp $exePath
Write-OK "Plainship v$ver ($goarch) installed to $exePath"

# ---- 6. 访问令牌 ----
$tokenFile = Join-Path $DataDir 'server.token'
if ((Test-Path $tokenFile) -and (Get-Item $tokenFile).Length -gt 0) {
    $token = (Get-Content $tokenFile -Raw).Trim()
    Write-Host 'Reusing existing access token'
} else {
    $bytes = New-Object byte[] 16
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
    $token = 'ps_' + (($bytes | ForEach-Object { $_.ToString('x2') }) -join '')
    Set-Content -Path $tokenFile -Value $token -Encoding ascii -NoNewline
    Write-Host 'Generated a new access token'
}

# ---- 7. 后台启动 ----
$logFile = Join-Path $DataDir 'plainship.log'
$errLogFile = Join-Path $DataDir 'plainship.err.log'

# 参数需手工加引号: Start-Process 在 PS 5.1 下会把数组 join 成命令行字符串,
# 含空格的路径会被拆散.
$serveArgs = @('serve', '--addr', $Addr, '--data', ('"{0}"' -f $DataDir))
$proc = Start-Process -FilePath $exePath -ArgumentList $serveArgs -WindowStyle Hidden `
    -RedirectStandardOutput $logFile -RedirectStandardError $errLogFile -PassThru
Set-Content -Path $pidFile -Value $proc.Id -Encoding ascii
Start-Sleep -Seconds 1
if ($proc.HasExited) { Write-Fail "Server failed to start, see logs: $logFile / $errLogFile" }
Write-OK "Server started in background (PID $($proc.Id), logs: $logFile / $errLogFile)"

# ---- 8. 输出 ----
# 取第一个非回环 IPv4 (Windows 主机名通常无 DNS 记录, 客户端连不上).
$hostName = 'localhost'
try {
    $addrInfo = [System.Net.Dns]::GetHostAddresses([System.Net.Dns]::GetHostName()) | Where-Object {
        $_.AddressFamily -eq [System.Net.Sockets.AddressFamily]::InterNetwork -and
        -not [System.Net.IPAddress]::IsLoopback($_.IPAddressToString)
    } | Select-Object -First 1
    if ($addrInfo) { $hostName = $addrInfo.IPAddressToString }
} catch { }

$port = $Addr.Substring($Addr.LastIndexOf(':') + 1)

Write-Host ''
Write-Host '===========================================================' -ForegroundColor Cyan
Write-Host " Plainship v$ver ready" -ForegroundColor Cyan
Write-Host ''
Write-Host "  Server URL: http://$hostName`:$port"
Write-Host "  Data dir:   $DataDir"
Write-Host ''
Write-Host '  Access token (copy this):'
Write-Host "  $token" -ForegroundColor Green
Write-Host ''
Write-Host '  On your client (in the Space dir) run:'
Write-Host "    plainship connect http://$hostName`:$port"
Write-Host '  then paste the token, and run plainship publish to publish'
Write-Host ''
Write-Host "  Forgot the token? On the server run: plainship token --data $DataDir"
Write-Host '===========================================================' -ForegroundColor Cyan
Write-Host ''
