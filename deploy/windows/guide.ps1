<#
.SYNOPSIS
  MeowFlixEmby 新手引导：首次设置 / 启动 / 开机自启，全中文交互。

.DESCRIPTION
  Backs the double-clickable .bat shims. The shims are deliberately pure ASCII
  because cmd.exe mis-parses multi-line UTF-8 Chinese in batch files (it tracks
  file offsets in characters, not bytes, under chcp 65001, which desynchronises
  the parser). All Chinese prompts therefore live here, in a UTF-8-with-BOM
  PowerShell script, which renders correctly regardless of the console codepage.

  Locates meowflix.exe in either layout:
    - release zip: exe sits next to the .bat shims at the archive root
    - git checkout: shims are in deploy\windows\, exe built into dist\ by
      scripts/build.sh as meowflix-<goos>-<goarch>.exe

.PARAMETER Action
  setup | start | autostart-install | autostart-uninstall

.PARAMETER ShimDir
  Directory holding the .bat shim that invoked this script. Passed as %~dp0 so
  path resolution follows the shim, not this file (they differ in the repo
  layout, where the shims sit at the repo root but this script is in deploy\windows\).
#>
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('setup', 'start', 'autostart-install', 'autostart-uninstall')]
    [string]$Action,

    [string]$ShimDir
)

$ErrorActionPreference = 'Stop'

if (-not $ShimDir) { $ShimDir = $PSScriptRoot }
# %~dp0 ends in a backslash, which escapes the closing quote when the shim
# passes it as -ShimDir "%~dp0" — PowerShell then sees a trailing quote in the
# value. Strip both so Join-Path gets a clean directory.
$ShimDir = $ShimDir.Trim('"').TrimEnd('\')

# Chinese output needs a UTF-8 console; the shim already ran chcp 65001, but set
# the .NET-side encoding too so Write-Host isn't re-encoded through cp936.
try { [Console]::OutputEncoding = [Text.Encoding]::UTF8 } catch { }

function Write-Banner([string]$Text) {
    Write-Host ''
    Write-Host '  ============================================' -ForegroundColor Cyan
    Write-Host "    $Text" -ForegroundColor Cyan
    Write-Host '  ============================================' -ForegroundColor Cyan
    Write-Host ''
}

function Write-Err([string]$Text) {
    Write-Host "  [错误] $Text" -ForegroundColor Red
}

function Write-Tip([string]$Text) {
    Write-Host "  $Text" -ForegroundColor Yellow
}

# Resolve meowflix.exe. Returns a hashtable with Exe / Root / Example, or exits
# with a user-readable message explaining the most common cause (running from
# inside the still-zipped archive).
function Resolve-Install {
    # Repo layout: the shims live at the repo root in a release zip, but during
    # development they sit in deploy\windows\, two levels below it.
    $repo = Resolve-Path -LiteralPath (Join-Path $ShimDir '..\..') -ErrorAction SilentlyContinue
    $bases = @($ShimDir)
    if ($repo) { $bases += $repo.Path }

    foreach ($base in $bases) {
        $candidates = @(
            # Release layout: everything at the archive root, next to the shim.
            @{ Exe = Join-Path $base 'meowflix.exe'
               Example = Join-Path $base 'meowflix.example.yaml' }
            # Repo layout: exe built into dist\ by scripts/build.sh.
            @{ Exe = Join-Path $base 'dist\meowflix.exe'
               Example = Join-Path $base 'configs\meowflix.example.yaml' }
            @{ Exe = Join-Path $base 'dist\meowflix-windows-amd64.exe'
               Example = Join-Path $base 'configs\meowflix.example.yaml' }
            @{ Exe = Join-Path $base 'dist\meowflix-windows-arm64.exe'
               Example = Join-Path $base 'configs\meowflix.example.yaml' }
        )
        foreach ($c in $candidates) {
            if (Test-Path -LiteralPath $c.Exe) {
                $exe = (Resolve-Path -LiteralPath $c.Exe).Path
                return @{
                    Exe     = $exe
                    Root    = Split-Path $exe -Parent
                    Example = $c.Example
                }
            }
        }
    }
    Write-Err '找不到 meowflix.exe。'
    Write-Host ''
    Write-Tip '最常见的原因：你在压缩包里直接双击了脚本。'
    Write-Tip '请先右键压缩包 →「全部解压缩」，解压到比如 C:\meowflix，'
    Write-Tip '再进入解压后的文件夹双击运行。'
    Write-Host ''
    exit 1
}

# Path of the config file that belongs to this installation.
function Get-ConfigPath($install) { Join-Path $install.Root 'meowflix.yaml' }

function Assert-Config($install) {
    $cfg = Get-ConfigPath $install
    if (-not (Test-Path -LiteralPath $cfg)) {
        Write-Err '还没有配置文件 meowflix.yaml。'
        Write-Host ''
        Write-Tip '请先双击「1-首次设置.bat」生成并填写配置。'
        Write-Host ''
        exit 1
    }
    return $cfg
}

function Invoke-Setup {
    Write-Banner 'MeowFlixEmby  首次设置'

    $install = Resolve-Install
    $cfg = Get-ConfigPath $install

    if (Test-Path -LiteralPath $cfg) {
        Write-Host '  已存在配置文件：'
        Write-Host "    $cfg"
        Write-Host ''
        Write-Host '  直接打开它继续编辑。想推倒重来，先删掉这个文件再运行本脚本。'
    }
    else {
        if (-not (Test-Path -LiteralPath $install.Example)) {
            Write-Err '找不到配置模板 meowflix.example.yaml。'
            Write-Tip '请确认压缩包已完整解压，且模板与 meowflix.exe 在同一个文件夹。'
            Write-Host ''
            exit 1
        }
        Copy-Item -LiteralPath $install.Example -Destination $cfg
        Write-Host '  已生成配置文件：'
        Write-Host "    $cfg"
    }

    Write-Host ''
    Write-Host '  接下来记事本会打开这个文件，至少要改这几处：'
    Write-Host ''
    Write-Host '    server.address    服务器地址，必须带 http:// 或 https:// 开头'
    Write-Host '    server.username   你的用户名'
    Write-Host '    server.password   你的密码，写在英文引号 "" 中间'
    Write-Host '    players.exe.mpv   你电脑上播放器的完整路径'
    Write-Host ''
    Write-Tip '注意：不要改动每行开头的空格（缩进），冒号后面要留一个空格。'
    Write-Tip '改完按 Ctrl+S 保存，然后关闭记事本，本窗口会继续。'
    Write-Host ''
    Read-Host '  按回车键打开记事本'

    Start-Process -FilePath notepad.exe -ArgumentList "`"$cfg`"" -Wait

    Write-Banner '设置完成。下一步：双击「2-启动.bat」'
}

function Invoke-Start {
    $install = Resolve-Install
    $cfg = Assert-Config $install

    Write-Banner 'MeowFlixEmby 启动中...'
    Write-Host '  看到 authenticated 就代表连接成功，'
    Write-Host '  然后到网页端点「投放 / Play On」选择本机。'
    Write-Host ''
    Write-Tip '这个窗口不要关闭。要停止请按 Ctrl+C。'
    Write-Host ''

    # Run in the exe's own directory so relative paths in the config (e.g.
    # log.file: ./meowflix.log) land next to the binary.
    Push-Location $install.Root
    try {
        & $install.Exe -config $cfg
        $code = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }

    Write-Host ''
    if ($code -eq 0) {
        Write-Host '  MeowFlixEmby 已退出。'
    }
    else {
        Write-Err "MeowFlixEmby 异常退出，错误码 $code。"
        Write-Tip '上面的英文提示说明了原因，对照 docs/00-新手入门指南.md 的'
        Write-Tip '「遇到问题怎么办」一节排查。'
    }
    return $code
}

# Path to setup-autostart.ps1, in either the release or repo layout.
function Resolve-AutostartScript {
    $candidates = @(
        (Join-Path $PSScriptRoot 'setup-autostart.ps1'),
        (Join-Path $ShimDir 'deploy\windows\setup-autostart.ps1'),
        (Join-Path $ShimDir 'setup-autostart.ps1')
    )
    foreach ($p in $candidates) {
        if (Test-Path -LiteralPath $p) { return (Resolve-Path -LiteralPath $p).Path }
    }
    Write-Err '找不到 setup-autostart.ps1，压缩包可能没有解压完整。'
    Write-Host ''
    exit 1
}

function Invoke-AutostartInstall {
    Write-Banner 'MeowFlixEmby  安装开机自启'

    $install = Resolve-Install
    $cfg = Assert-Config $install
    $ps1 = Resolve-AutostartScript

    Write-Host '  将注册为当前用户的登录启动项：'
    Write-Host "    程序：$($install.Exe)"
    Write-Host "    配置：$cfg"
    Write-Host ''
    Write-Host '  下次登录 Windows 时自动在后台运行，不弹窗口，不需要管理员权限。'
    Write-Tip "窗口隐藏后日志看这里：$(Join-Path $install.Root 'meowflix.log')"
    Write-Host ''
    Read-Host '  按回车键开始安装'
    Write-Host ''

    # A `throw` inside the child script propagates out of the call, so there is
    # nothing to check afterwards. ($LASTEXITCODE is not usable here: it is only
    # set by native executables and stays empty after a PowerShell script.)
    # It reports in English via Write-Host, i.e. on the information stream, so
    # redirect 6> as well to keep this guide's output Chinese-only.
    & $ps1 -Action install -Exe $install.Exe -Config $cfg 6>$null | Out-Null

    Write-Banner '安装完成。重启或重新登录后生效。'
    Write-Host '  现在就想用，可以直接双击「2-启动.bat」。'
    Write-Host '  不想要了，双击「3-开机自启-卸载.bat」。'
    Write-Host ''
}

function Invoke-AutostartUninstall {
    Write-Banner 'MeowFlixEmby  卸载开机自启'

    $ps1 = Resolve-AutostartScript

    Write-Host '  将移除当前用户的登录启动项。'
    Write-Host '  程序和配置文件都不会被删除，之后仍可双击「2-启动.bat」手动运行。'
    Write-Host ''
    Read-Host '  按回车键确认移除'
    Write-Host ''

    & $ps1 -Action uninstall 6>$null | Out-Null

    Write-Banner '已移除。下次开机不会再自动启动。'
    Write-Tip '本次已在运行的进程不会被结束，重启电脑后即彻底停止。'
    Write-Host ''
}

switch ($Action) {
    'setup'               { Invoke-Setup; exit 0 }
    'start'               { exit (Invoke-Start) }
    'autostart-install'   { Invoke-AutostartInstall; exit 0 }
    'autostart-uninstall' { Invoke-AutostartUninstall; exit 0 }
}
