<#
.SYNOPSIS
  Install, remove, or control the MeowFlixEmby Windows service.

.DESCRIPTION
  Thin wrapper around the daemon's built-in service control (meowflix.exe
  -service ...). Must be run from an elevated (Administrator) PowerShell, since
  registering a service requires admin rights.

.PARAMETER Action
  install | uninstall | start | stop | status   (default: install)

.PARAMETER Exe
  Path to meowflix.exe. Defaults to meowflix.exe next to this script.

.PARAMETER Config
  Path to the config file. Defaults to meowflix.yaml next to the exe.

.EXAMPLE
  # From an elevated prompt:
  .\install-service.ps1 -Action install -Exe C:\meowflix\meowflix.exe -Config C:\meowflix\meowflix.yaml
  .\install-service.ps1 -Action start
  .\install-service.ps1 -Action status
#>
param(
    [ValidateSet('install', 'uninstall', 'start', 'stop', 'status')]
    [string]$Action = 'install',
    [string]$Exe,
    [string]$Config
)

$ErrorActionPreference = 'Stop'

# Require elevation for actions that change service state.
$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "This script must be run as Administrator."
}

if (-not $Exe) {
    $Exe = Join-Path $PSScriptRoot 'meowflix.exe'
}
if (-not (Test-Path $Exe)) {
    throw "meowflix.exe not found at '$Exe'. Pass -Exe <path>."
}

$exeArgs = @('-service', $Action)
if ($Action -eq 'install') {
    if (-not $Config) {
        $Config = Join-Path (Split-Path $Exe -Parent) 'meowflix.yaml'
    }
    $exeArgs += @('-config', $Config)
}

Write-Host "Running: $Exe $($exeArgs -join ' ')"
& $Exe @exeArgs
exit $LASTEXITCODE
