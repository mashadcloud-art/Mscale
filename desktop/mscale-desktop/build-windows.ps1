# Build MScale desktop with admin manifest and versioned exe in build\bin.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

# Bump when you ship a new build (also in wails.json info.productVersion).
$AppVersion = "0.4.7"
$wailsJson = Join-Path $PSScriptRoot "wails.json"
if (Test-Path $wailsJson) {
    $raw = Get-Content $wailsJson -Raw
    if ($raw -match '"productVersion"\s*:\s*"([^"]+)"') {
        $AppVersion = $Matches[1]
    }
}
$BuildStamp = Get-Date -Format "yyyyMMdd-HHmm"
$VersionedExe = "mscale-desktop-v${AppVersion}-${BuildStamp}.exe"

Write-Host "MScale desktop build version: $AppVersion"
Write-Host "Output name: $VersionedExe"
Write-Host ""

Write-Host "Building frontend..."
Push-Location frontend
npm install --silent 2>$null
npm run build
Pop-Location

Write-Host "Building Windows binary (requires Administrator manifest)..."
wails build -platform windows/amd64

$binDir = Join-Path $PSScriptRoot "build\bin"
$exe = Join-Path $binDir "mscale-desktop.exe"
$nested = Join-Path $binDir "build\bin\mscale-desktop.exe"
if (-not (Test-Path $exe) -and (Test-Path $nested)) {
    Copy-Item $nested $exe -Force
}
if (-not (Test-Path $exe)) {
    Write-Error "Build failed: mscale-desktop.exe not found in $binDir"
}

$dll = Join-Path $PSScriptRoot "third_party\wintun\amd64\wintun.dll"
if (-not (Test-Path $dll)) {
    Write-Host "Downloading wintun.dll from wintun.net..."
    $zip = Join-Path $env:TEMP "wintun-0.14.1.zip"
    Invoke-WebRequest -Uri "https://www.wintun.net/builds/wintun-0.14.1.zip" -OutFile $zip -UseBasicParsing
    Expand-Archive -Path $zip -DestinationPath (Join-Path $env:TEMP "wintun-extract") -Force
    $src = Join-Path $env:TEMP "wintun-extract\wintun\bin\amd64\wintun.dll"
    New-Item -ItemType Directory -Force -Path (Split-Path $dll) | Out-Null
    Copy-Item $src $dll -Force
}
try {
    Copy-Item $dll $binDir -Force -ErrorAction Stop
    Write-Host "wintun.dll copied to $binDir"
} catch {
    Write-Host "WARN: Could not copy wintun.dll (file in use). Close MScale, then rerun this script or copy manually."
    if (-not (Test-Path (Join-Path $binDir "wintun.dll"))) {
        Write-Host "ERROR: wintun.dll missing in $binDir - exit MScale and rerun."
        exit 1
    }
}

$versionedPath = Join-Path $binDir $VersionedExe
$latestPath = Join-Path $binDir "mscale-desktop-v${AppVersion}.exe"
Copy-Item $exe $versionedPath -Force
Copy-Item $exe $latestPath -Force

$infoPath = Join-Path $binDir "BUILD_INFO.txt"
@"
MScale Desktop
Version: $AppVersion
Built:   $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')
Run:     $VersionedExe  (as Administrator)
Also:    mscale-desktop-v${AppVersion}.exe (same build, no timestamp)
Requires wintun.dll in this folder.
"@ | Set-Content $infoPath -Encoding ASCII

Write-Host ""
Write-Host "Done."
Write-Host "  Folder:   $binDir"
Write-Host "  Newest:   $VersionedExe"
Write-Host "  Version:  mscale-desktop-v${AppVersion}.exe"
Write-Host "  Info:     BUILD_INFO.txt"
Write-Host "Run as Administrator - UAC prompt expected for VPN."
