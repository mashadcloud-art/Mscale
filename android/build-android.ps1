# build-android.ps1
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

# Ensure gomobile is in path
$env:PATH += ";C:\Users\PC\go\bin"
$env:ANDROID_HOME = "C:\Users\PC\AppData\Local\Android\Sdk"
$env:ANDROID_NDK_HOME = "C:\Users\PC\AppData\Local\Android\Sdk\ndk\27.1.12297006"

Write-Host "Initializing Gomobile..."
gomobile init

Write-Host "Building Android Archive (.aar)..."
# Navigate to the core module
Set-Location "core"

# Go mod tidy to resolve dependencies
go get -tool golang.org/x/mobile/cmd/gobind
go mod tidy

# Bind the package for android
gomobile bind -target=android -androidapi 24 -o mscalecore.aar

Write-Host "Build Complete!"
Write-Host "Output: $(Join-Path $PWD 'mscalecore.aar')"
