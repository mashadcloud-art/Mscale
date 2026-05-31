# Mscale Windows installer — runs elevated from IExpress or NSIS post-step.
$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Windows.Forms

function Test-Admin {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    $p = New-Object Security.Principal.WindowsPrincipal($id)
    return $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Ensure-Admin {
    if (Test-Admin) { return }
    $arg = "-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$PSCommandPath`""
    Start-Process powershell.exe -Verb RunAs -WindowStyle Hidden -ArgumentList $arg -Wait
    exit 0
}

Ensure-Admin

$payloadDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$appExe = Get-ChildItem $payloadDir -Filter "mscale-desktop*.exe" | Select-Object -First 1
if (-not $appExe) {
    [System.Windows.Forms.MessageBox]::Show(
        "Installer payload missing. Please download again from mashad.shop/mscale/download.html",
        "Mscale Setup", "OK", "Error") | Out-Null
    exit 1
}

$installDir = Join-Path ${env:ProgramFiles} "Mscale"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Copy-Item $appExe.FullName (Join-Path $installDir $appExe.Name) -Force

$targetExe = Join-Path $installDir $appExe.Name

# WebView2 runtime (required by Wails)
$wv2Key = "HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
$wv2 = Get-ItemProperty -Path $wv2Key -Name "pv" -ErrorAction SilentlyContinue
if (-not $wv2.pv) {
    $bootstrap = Join-Path $payloadDir "MicrosoftEdgeWebview2Setup.exe"
    if (Test-Path $bootstrap) {
        Start-Process -FilePath $bootstrap -ArgumentList "/silent", "/install" -Wait -NoNewWindow
    }
}

$shell = New-Object -ComObject WScript.Shell
$programs = [Environment]::GetFolderPath("CommonPrograms")
$startLnk = Join-Path $programs "Mscale.lnk"
$desktopLnk = Join-Path ([Environment]::GetFolderPath("CommonDesktopDirectory")) "Mscale.lnk"

foreach ($lnk in @($startLnk, $desktopLnk)) {
    $sc = $shell.CreateShortcut($lnk)
    $sc.TargetPath = $targetExe
    $sc.WorkingDirectory = $installDir
    $sc.Description = "Mscale VPN"
    $sc.Save()
}

# Uninstaller stub
$uninstall = @"
Remove-Item -Recurse -Force '$installDir' -ErrorAction SilentlyContinue
Remove-Item '$startLnk','$desktopLnk' -Force -ErrorAction SilentlyContinue
"@
$uninstallPath = Join-Path $installDir "uninstall.ps1"
Set-Content -Path $uninstallPath -Value $uninstall -Encoding UTF8

$launch = [System.Windows.Forms.MessageBox]::Show(
    "Mscale was installed successfully.`n`nOpen Mscale now?",
    "Mscale Setup", "YesNo", "Information")
if ($launch -eq "Yes") {
    Start-Process -FilePath $targetExe
}
exit 0
