# Build MScale desktop with admin manifest and versioned exe in build\bin.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

# Bump when you ship a new build (also in wails.json info.productVersion).
$AppVersion = "0.4.7"
$OutputFilename = "mscale-desktop"
$wailsJson = Join-Path $PSScriptRoot "wails.json"
if (Test-Path $wailsJson) {
    $raw = Get-Content $wailsJson -Raw
    if ($raw -match '"productVersion"\s*:\s*"([^"]+)"') {
        $AppVersion = $Matches[1]
    }
    if ($raw -match '"outputfilename"\s*:\s*"([^"]+)"') {
        $OutputFilename = $Matches[1]
    }
}
$BuildStamp = Get-Date -Format "yyyyMMdd-HHmm"
$VersionedExe = "mscale-desktop-v${AppVersion}-${BuildStamp}.exe"
$WailsOutputExe = "$OutputFilename.exe"

Write-Host "MScale desktop build version: $AppVersion"
Write-Host "Output name: $VersionedExe"
Write-Host ""

Write-Host "Building frontend..."
Push-Location frontend
npm install --silent 2>$null
npm run build
Pop-Location

Write-Host "Building Windows binary..."
$env:Path = "$env:USERPROFILE\go\bin;C:\Program Files\Go\bin;" + $env:Path
$ldflags = "-X main.appVersion=$AppVersion"
$makensis = Get-Command makensis -ErrorAction SilentlyContinue
if (-not $makensis) {
    $nsisDefault = "C:\Program Files (x86)\NSIS\makensis.exe"
    if (Test-Path $nsisDefault) { $makensis = Get-Command $nsisDefault }
}
if ($makensis) {
    Write-Host "NSIS found — building with Wails installer..."
    wails build -platform windows/amd64 -ldflags $ldflags -nsis
} else {
    Write-Host "NSIS not found — building portable exe (IExpress installer added after build)..."
    wails build -platform windows/amd64 -ldflags $ldflags
}

$binDir = Join-Path $PSScriptRoot "build\bin"
$exe = Join-Path $binDir $WailsOutputExe
$nested = Join-Path $binDir "build\bin\$WailsOutputExe"
if (-not (Test-Path $exe) -and (Test-Path $nested)) {
    Copy-Item $nested $exe -Force
}
if (-not (Test-Path $exe)) {
    $fallback = Join-Path $binDir "mscale-desktop.exe"
    if (Test-Path $fallback) { $exe = $fallback }
}
if (-not (Test-Path $exe)) {
    Write-Error "Build failed: $WailsOutputExe not found in $binDir"
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
$canonicalPath = Join-Path $binDir "mscale-desktop.exe"
$tempBuilt = Join-Path $env:TEMP "mscale-desktop-build-$BuildStamp.exe"

Copy-Item $exe $tempBuilt -Force
Get-ChildItem $binDir -Filter "mscale-desktop*.exe" | Remove-Item -Force -ErrorAction SilentlyContinue
Copy-Item $tempBuilt $versionedPath -Force
Copy-Item $tempBuilt $latestPath -Force
Copy-Item $tempBuilt $canonicalPath -Force
Remove-Item $tempBuilt -Force -ErrorAction SilentlyContinue

$setupName = "MscaleSetup-v${AppVersion}.exe"
$setupPath = Join-Path $binDir $setupName
$installer = Get-ChildItem $binDir -Filter "*-installer.exe" -ErrorAction SilentlyContinue | Sort-Object LastWriteTime -Descending | Select-Object -First 1
if ($installer) {
    Copy-Item $installer.FullName $setupPath -Force
    Write-Host "Installer (NSIS): $setupName"
} else {
    Write-Host "Building IExpress installer..."
    $iexpressScript = Join-Path $PSScriptRoot "build\installer\build-iexpress.ps1"
    & $iexpressScript -AppVersion $AppVersion -PayloadExe $latestPath -OutFile $setupPath
}

$serverDl = Join-Path $PSScriptRoot "..\..\server\downloads"
New-Item -ItemType Directory -Force -Path $serverDl | Out-Null
Copy-Item $latestPath (Join-Path $serverDl "mscale-desktop-v${AppVersion}.exe") -Force
Copy-Item $dll (Join-Path $serverDl "wintun.dll") -Force
if (Test-Path (Join-Path $binDir $setupName)) {
    Copy-Item (Join-Path $binDir $setupName) (Join-Path $serverDl $setupName) -Force
}

$infoPath = Join-Path $binDir "BUILD_INFO.txt"
@"
MScale Desktop
Version: $AppVersion
Built:   $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')
Run:     $VersionedExe  (as Administrator)
Setup:   $setupName  (recommended — installs to Program Files)
Also:    mscale-desktop-v${AppVersion}.exe (portable)
Requires wintun.dll in this folder (portable) or auto-installed with setup.
"@ | Set-Content $infoPath -Encoding ASCII

Write-Host ""
Write-Host "Done."
Write-Host "  Folder:   $binDir"
Write-Host "  Newest:   $VersionedExe"
Write-Host "  Version:  mscale-desktop-v${AppVersion}.exe"
Write-Host "  Info:     BUILD_INFO.txt"
Write-Host "Run as Administrator - UAC prompt expected for VPN."
