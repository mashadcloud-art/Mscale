param(
    [Parameter(Mandatory = $true)][string]$AppVersion,
    [Parameter(Mandatory = $true)][string]$PayloadExe,
    [Parameter(Mandatory = $true)][string]$OutFile
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$staging = Join-Path $env:TEMP "mscale-iexpress-$AppVersion"
if (Test-Path $staging) { Remove-Item $staging -Recurse -Force }
New-Item -ItemType Directory -Force -Path $staging | Out-Null

Copy-Item $PayloadExe (Join-Path $staging (Split-Path -Leaf $PayloadExe)) -Force
Copy-Item (Join-Path $root "install.ps1") (Join-Path $staging "install.ps1") -Force
Copy-Item (Join-Path $root "install.vbs") (Join-Path $staging "install.vbs") -Force

$wv2 = Join-Path $root "..\windows\installer\tmp\MicrosoftEdgeWebview2Setup.exe"
if (Test-Path $wv2) {
    Copy-Item $wv2 (Join-Path $staging "MicrosoftEdgeWebview2Setup.exe") -Force
}

$exeName = Split-Path -Leaf $PayloadExe
$sedPath = Join-Path $staging "mscale.sed"
$outDir = Split-Path -Parent $OutFile
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

$files = @(
    "FILE0=$exeName",
    "FILE1=install.ps1",
    "FILE2=install.vbs"
)
$sourceEntries = @("%FILE0%=", "%FILE1%=", "%FILE2%=")
$fileIdx = 3
if (Test-Path (Join-Path $staging "MicrosoftEdgeWebview2Setup.exe")) {
    $files += "FILE$fileIdx=MicrosoftEdgeWebview2Setup.exe"
    $sourceEntries += "%FILE$fileIdx%="
}

$sed = @"
[Version]
Class=IEXPRESS
SEDVersion=3
[Options]
PackagePurpose=InstallApp
ShowInstallProgramWindow=0
HideExtractAnimation=1
UseLongFileName=1
InsideCompressed=1
CAB_FixedSize=0
CAB_ResvCodeSigning=0
RebootMode=N
InstallPrompt=%InstallPrompt%
DisplayLicense=
FinishMessage=%FinishMessage%
TargetName=%TargetName%
FriendlyName=%FriendlyName%
AppLaunched=%AppLaunched%
PostInstallCmd=<None>
AdminQuietInstCmd=
UserQuietInstCmd=
SourceFiles=SourceFiles
[Strings]
InstallPrompt=Install Mscale v$AppVersion on this computer?
FinishMessage=Installing Mscale…
FriendlyName=Mscale Setup v$AppVersion
TargetName=$OutFile
AppLaunched=wscript.exe //B //Nologo install.vbs
$($files -join "`n")
[SourceFiles]
SourceFiles0=$staging\
[SourceFiles0]
$($sourceEntries -join "`n")
"@

Set-Content -Path $sedPath -Value $sed -Encoding ASCII

$iexpress = Join-Path $env:WINDIR "System32\iexpress.exe"
if (-not (Test-Path $iexpress)) {
    throw "IExpress not found at $iexpress"
}

if (Test-Path $OutFile) { Remove-Item $OutFile -Force }

$p = Start-Process -FilePath $iexpress -ArgumentList "/N", "/Q", $sedPath -Wait -PassThru -NoNewWindow
if ($p.ExitCode -ne 0 -or -not (Test-Path $OutFile)) {
    throw "IExpress build failed (exit $($p.ExitCode))"
}

Write-Host "Built installer: $OutFile ($([math]::Round((Get-Item $OutFile).Length/1MB, 2)) MB)"
