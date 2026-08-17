# deploy.ps1 - Macros Windows deploy script
# Builds the Windows binary + NSIS installer, computes checksums and
# publishes a GitHub Release (repo: beyhano/macros).
#
# Requirements: PowerShell, git, gh, wails3, NSIS (makensis).
# Run from the project root:
#   powershell -ExecutionPolicy Bypass -File .\deploy.ps1
# ASCII-only text (avoids PowerShell 5.1 ANSI encoding pitfalls).

[CmdletBinding()]
param()
$ErrorActionPreference = "Stop"

# Runs a native command tolerantly: PS 5.1 turns native stderr into a
# terminating NativeCommandError when $ErrorActionPreference=Stop, so we
# relax it around every external call and check the exit code ourselves.
function Invoke-Native {
    param([string]$What, [scriptblock]$Cmd)
    $old = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try { & $Cmd 2>&1 | Out-Host } finally { $ErrorActionPreference = $old }
    $exit = $LASTEXITCODE
    if ($exit -ne 0) { throw "$What basarisiz (exit $exit)" }
}

function Test-NativeOk {
    param([scriptblock]$Cmd)
    $old = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try { & $Cmd 2>&1 | Out-Null } finally { $ErrorActionPreference = $old }
    return ($LASTEXITCODE -eq 0)
}

$Root = (Get-Location).Path
Write-Host "macros deploy basliyor..." -ForegroundColor Cyan

# 1. Version
$manifest = Get-Content -Raw (Join-Path $Root "version.json") | ConvertFrom-Json
$Version = $manifest.current.version
if (-not $Version) { throw "version.json icinde current.version bulunamadi!" }
Write-Host "Versiyon: v$Version"

# 1b. GitHub auth preflight
Write-Host "gh auth kontrol..."
if (-not (Test-NativeOk { & gh auth status })) {
    Write-Host ""
    Write-Host "HATA: gh ile giris yapilmamis." -ForegroundColor Red
    Write-Host "Once su komutu calistir ve GitHub hesabina giris yap:"
    Write-Host "   gh auth login"
    Write-Host "Sonra bu scripti tekrar calistir."
    exit 1
}

# 2. Commit & push
Write-Host "Commit & push..."
Invoke-Native "git add"       { git add . }
Invoke-Native "git commit"    { git commit -m "v$Version" --allow-empty }
Invoke-Native "git push"      { git push }

# 3. Build Windows binary
Write-Host "Windows binary build..."
Invoke-Native "wails3 windows:build" { & wails3 task windows:build }
$Exe = Join-Path $Root "bin\macros.exe"
if (-not (Test-Path $Exe)) { throw "build ciktisi yok: $Exe" }

# 4. NSIS installer (per-user scope so the integrated updater can swap the exe)
Write-Host "Windows NSIS installer (per-user)..."
Invoke-Native "wails3 windows:package" { & wails3 task windows:package INSTALL_SCOPE=user }
$amd64Installer = Join-Path $Root "bin\macros-amd64-installer.exe"
if (Test-Path $amd64Installer) {
    Copy-Item $amd64Installer (Join-Path $Root "bin\macros-installer.exe") -Force
    Write-Host "   OK Installer: bin\macros-installer.exe"
} else {
    Write-Host "   WARN Installer olusturulamadi" -ForegroundColor Yellow
}

# 5. SHA256SUMS sidecar (filenames MUST match the release asset basenames)
Write-Host "SHA256SUMS uretiliyor..."
$sumLines = @()
foreach ($f in @("macros.exe", "macros-installer.exe")) {
    $p = Join-Path $Root "bin\$f"
    if (-not (Test-Path $p)) { continue }
    $hash = (Get-FileHash -Algorithm SHA256 -Path $p).Hash.ToLowerInvariant()
    $sumLines += "$hash  $f"
}
Set-Content -Path (Join-Path $Root "bin\SHA256SUMS") -Value $sumLines -Encoding ascii

# 6. Release assets (only files that actually exist)
$Assets = @()
foreach ($a in @(
        @{ Path = "bin\macros.exe";             Label = "macros-windows-amd64.exe" }
        @{ Path = "bin\macros-installer.exe";   Label = "macros-installer.exe" }
        @{ Path = "bin\SHA256SUMS";             Label = "SHA256SUMS" }
        @{ Path = "bin\macros.AppImage";        Label = "macros-linux-amd64.AppImage" }
        @{ Path = "bin\macros";                 Label = "macros-linux-amd64" }
    )) {
    if (Test-Path (Join-Path $Root $a.Path)) {
        $Assets += "./$($a.Path)#$($a.Label)"
    }
}

# 7. Create or update the GitHub Release
Write-Host "GitHub Release gonderiliyor..."
$Changelog = "Yeni surum"
if ($manifest.history -and $manifest.history.Count -gt 0) {
    $Changelog = $manifest.history[0].changelog
}

if (Test-NativeOk { & gh release view "v$Version" }) {
    Invoke-Native "gh release upload" { & gh release upload "v$Version" @Assets --clobber }
} else {
    Invoke-Native "gh release create" { & gh release create "v$Version" --title "v$Version" --notes $Changelog @Assets }
}

Write-Host ""
Write-Host "OK v$Version yayinlandi!" -ForegroundColor Green
Write-Host "   https://github.com/beyhano/macros/releases/tag/v$Version"