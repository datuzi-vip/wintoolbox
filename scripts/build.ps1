# WinToolbox one-click build (Go + Wails + Vue3)
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot\..

function Assert-LastExitCode([string]$step) {
  if ($null -ne $LASTEXITCODE -and $LASTEXITCODE -ne 0) {
    throw "$step failed with exit code $LASTEXITCODE"
  }
}

if (-not (Test-Path assets\app.ico)) {
  Write-Host '==> Generating assets\app.ico' -ForegroundColor Cyan
  python .\scripts\gen-icon.py
  Assert-LastExitCode 'gen-icon.py'
}

Write-Host '==> Syncing version from version.json' -ForegroundColor Cyan
& $PSScriptRoot\sync-version.ps1

Write-Host '==> Building frontend (Vue3 + Element Plus)' -ForegroundColor Cyan
Push-Location frontend
try {
  if (-not (Test-Path node_modules)) {
    npm install
    Assert-LastExitCode 'npm install'
  }
  npm run build
  Assert-LastExitCode 'npm run build'
} finally {
  Pop-Location
}

Write-Host '==> Embedding manifest + icon' -ForegroundColor Cyan
go run github.com/akavel/rsrc@latest -manifest app.manifest -ico assets/app.ico -o rsrc.syso -arch amd64
Assert-LastExitCode 'rsrc'

Write-Host '==> Building WinToolbox.exe (Wails)' -ForegroundColor Cyan
# Wails requires desktop,production tags for a real window (plain go build shows a stub error dialog).
go build -tags 'desktop,production' -trimpath -ldflags='-s -w -H windowsgui' -o WinToolbox.exe .
Assert-LastExitCode 'go build'

$size = (Get-Item WinToolbox.exe).Length
Write-Host ("Done: WinToolbox.exe ({0:N0} bytes / {1:N2} MB)" -f $size, ($size / 1MB)) -ForegroundColor Green
