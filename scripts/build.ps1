# Plainship 一键交叉编译脚本
#
# 用法:
#   ./scripts/build.ps1                   # 默认编译 linux / windows / macos (amd64)
#   ./scripts/build.ps1 -Arch arm64       # 指定目标架构 (例如苹果 M 系列)
#   ./scripts/build.ps1 -OutputDir dist   # 指定输出目录 (默认 bin)
#
# 产物:
#   bin/plainship-linux-amd64
#   bin/plainship-windows-amd64.exe
#   bin/plainship-darwin-amd64

param(
    [string]$Arch = "amd64",
    [string]$OutputDir = "bin"
)

$ErrorActionPreference = "Stop"

# 1. 定位仓库根目录 (脚本位于 <根>/scripts).
$repoRoot = Split-Path -Parent $PSScriptRoot

# 2. 检查 Go 是否可用 (PATH 中找不到时探测常见安装位置).
$goExe = $null
$cmdGo = Get-Command go -ErrorAction SilentlyContinue
if ($cmdGo) {
    $goExe = $cmdGo.Source
} else {
    $candidates = @(
        (Join-Path $env:ProgramFiles "Go\bin\go.exe"),
        (Join-Path ${env:ProgramFiles(x86)} "Go\bin\go.exe"),
        (Join-Path $env:LOCALAPPDATA "Programs\Go\bin\go.exe"),
        "D:\Program\Go\bin\go.exe",
        (Join-Path $env:USERPROFILE "scoop\apps\go\current\bin\go.exe"),
        (Join-Path $env:USERPROFILE "go\bin\go.exe")
    )
    foreach ($c in $candidates) {
        if (Test-Path -LiteralPath $c) {
            $goExe = $c
            break
        }
    }
}
if (-not $goExe) {
    Write-Error "go not found; install Go first: https://go.dev/dl/"
    exit 1
}

# 3. 准备输出目录.
$out = Join-Path $repoRoot $OutputDir
New-Item -ItemType Directory -Force -Path $out | Out-Null

Push-Location $repoRoot
try {
    # 记录原始环境变量, 脚本结束后恢复.
    $origCGO = $env:CGO_ENABLED
    $origGOOS = $env:GOOS
    $origGOARCH = $env:GOARCH

    # 关闭 CGO, 生成静态可执行文件, 便于跨平台分发.
    $env:CGO_ENABLED = "0"

    # 目标矩阵: 操作系统 -> 可执行文件名.
    $targets = @(
        @{ OS = "linux";   Name = "plainship-linux-$Arch" },
        @{ OS = "windows"; Name = "plainship-windows-$Arch.exe" },
        @{ OS = "darwin";  Name = "plainship-darwin-$Arch" }
    )

    foreach ($t in $targets) {
        $env:GOOS = $t.OS
        $env:GOARCH = $Arch
        $dest = Join-Path $out $t.Name
        Write-Host ("building {0}/{1} -> {2}" -f $t.OS, $Arch, $dest) -ForegroundColor Cyan
        & $goExe build -trimpath -o $dest ./cmd/plainship
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
    }

    Write-Host ""
    Write-Host ("build complete, artifacts in: {0}" -f $out) -ForegroundColor Green
    Get-ChildItem $out -Filter "plainship-*" | ForEach-Object {
        Write-Host ("  {0}  ({1:N0} KB)" -f $_.Name, ($_.Length / 1KB))
    }
}
finally {
    $env:CGO_ENABLED = $origCGO
    $env:GOOS = $origGOOS
    $env:GOARCH = $origGOARCH
    Pop-Location
}
