# Bypass the Wails desktop entirely.
# Generates a WireGuard config for the OFFICIAL WireGuard for Windows client
# that connects directly to the UAE hub (ph) and exits via India (pilehead).
#
# Usage (PowerShell, NOT as admin needed for this script):
#   .\quick-exit-via.ps1
# Then in WireGuard for Windows: Import tunnel from file -> select the .conf -> Activate.

$ErrorActionPreference = "Stop"

# --- Constants (do NOT change unless hub IP / pubkey changes) -----------------
$HubIP      = "129.151.146.44"
$HubPort    = 51820
$HubPubKey  = "uUxlKPDc9t8sHf4iEunMcA0+Z2Mt/LIULDeAcFTpGSM="
$ClientIP   = "100.64.1.11"     # your reserved overlay slot
$ClientMask = "10"
$ExitOverlay = "100.64.1.2"      # India exit node overlay (informational)
$OutDir     = "$env:USERPROFILE\Desktop"
$ConfName   = "mscale-india-exit.conf"

# --- Locate wg.exe ------------------------------------------------------------
$wg = "C:\Program Files\WireGuard\wg.exe"
if (-not (Test-Path $wg)) { throw "wg.exe not found. Install WireGuard for Windows from https://www.wireguard.com/install/" }

# --- Generate persistent keypair ---------------------------------------------
$keyDir = Join-Path $env:LOCALAPPDATA "mscale"
New-Item -ItemType Directory -Force -Path $keyDir | Out-Null
$privPath = Join-Path $keyDir "manual_private.key"
$pubPath  = Join-Path $keyDir "manual_public.key"

if (-not (Test-Path $privPath)) {
    Write-Host "Generating new WireGuard keypair..."
    $priv = & $wg genkey
    $priv | Set-Content -Path $privPath -NoNewline -Encoding ASCII
    $pub = $priv | & $wg pubkey
    $pub | Set-Content -Path $pubPath -NoNewline -Encoding ASCII
}
$ClientPriv = (Get-Content $privPath -Raw).Trim()
$ClientPub  = (Get-Content $pubPath -Raw).Trim()

Write-Host "Client public key (give this to the hub):"
Write-Host "  $ClientPub"
Write-Host ""

$applyScript = Join-Path $keyDir "apply-india-routing.ps1"
Copy-Item (Join-Path $PSScriptRoot "apply-india-routing.ps1") $applyScript -Force

# --- Write .conf for WireGuard for Windows -----------------------------------
$conf = @"
[Interface]
PrivateKey = $ClientPriv
Address = $ClientIP/$ClientMask
DNS = 8.8.8.8, 8.8.4.4
PostUp = powershell.exe -ExecutionPolicy Bypass -WindowStyle Hidden -File "$applyScript"

[Peer]
PublicKey = $HubPubKey
Endpoint = ${HubIP}:${HubPort}
AllowedIPs = 0.0.0.0/1, 128.0.0.0/1, 100.64.0.0/10
PersistentKeepalive = 25
"@

$confPath = Join-Path $OutDir $ConfName
$conf | Set-Content -Path $confPath -Encoding ASCII
Write-Host "Wrote tunnel config: $confPath"
Write-Host ""

# --- Hub commands -------------------------------------------------------------
$hubScript = @"
# Run on UAE hub (ph) as root or with sudo
sudo wg set wg0 peer $ClientPub allowed-ips $ClientIP/32 persistent-keepalive 25
sudo wg set wg0 peer dImB48LL2IjKtPhJYlITFmYihA+6wZRVC71Zg1CRKh8= \
  allowed-ips $ExitOverlay/32,0.0.0.0/1,128.0.0.0/1 persistent-keepalive 25
sudo ip rule del from $ClientIP/32 2>/dev/null
sudo ip rule add pref 100 from $ClientIP/32 lookup 100
sudo ip route replace 0.0.0.0/1 dev wg0 table 100
sudo ip route replace 128.0.0.0/1 dev wg0 table 100
sudo iptables -t nat -C POSTROUTING -o wg0 -j MASQUERADE 2>/dev/null || \
  sudo iptables -t nat -A POSTROUTING -o wg0 -j MASQUERADE
sudo iptables -C FORWARD -i wg0 -o wg0 -j ACCEPT 2>/dev/null || \
  sudo iptables -A FORWARD -i wg0 -o wg0 -j ACCEPT
sudo sysctl -w net.ipv4.ip_forward=1
sudo ip route get 8.8.8.8 from $ClientIP iif wg0
"@

$hubPath = Join-Path $OutDir "hub-commands.sh"
$hubScript | Set-Content -Path $hubPath -Encoding ASCII
Write-Host "Hub commands written to: $hubPath"
Write-Host ""

# --- Try SSH auto-apply -------------------------------------------------------
$ssh = "$env:WINDIR\System32\OpenSSH\ssh.exe"
if (Test-Path $ssh) {
    Write-Host "Attempting to SSH to ph and apply hub commands..."
    $script = Get-Content $hubPath -Raw
    $script | & $ssh -o ConnectTimeout=8 ph "bash -s" 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "SSH failed. Paste the commands from $hubPath into the hub manually." -ForegroundColor Yellow
    } else {
        Write-Host "Hub provisioned successfully." -ForegroundColor Green
    }
} else {
    Write-Host "OpenSSH not found locally. Paste the commands from $hubPath into the hub manually." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "NEXT STEPS:" -ForegroundColor Cyan
Write-Host "  1. Open WireGuard for Windows (Start menu -> WireGuard)"
Write-Host "  2. Click 'Import tunnel(s) from file' -> select: $confPath"
Write-Host "  3. Click 'Activate' on the imported tunnel"
Write-Host "  4. Open https://ifconfig.me -> should show ~129.159.x.x (India)"
Write-Host ""
Write-Host "If still UAE: hub commands did not run. Open Oracle Cloud Console -> ph instance -> Cloud Shell, paste contents of $hubPath."
