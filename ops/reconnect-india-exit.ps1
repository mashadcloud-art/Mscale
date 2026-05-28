# Applies India exit routing on UAE hub (run as normal user; needs network to ph).
$ErrorActionPreference = "Stop"
$keyDir = Join-Path $env:LOCALAPPDATA "mscale"
$overlay = "100.64.1.11"
$pub = ""

$wg = "C:\Program Files\WireGuard\wg.exe"
$privPath = Join-Path $keyDir "wg_private.key"
if ((Test-Path $wg) -and (Test-Path $privPath)) {
    $priv = (Get-Content $privPath -Raw).Trim()
    $pub = ($priv | & $wg pubkey).Trim()
    Write-Host "Using desktop app key from $privPath"
}

if (-not $pub) {
    $pubPath = Join-Path $keyDir "manual_public.key"
    if (Test-Path $pubPath) {
        $pub = (Get-Content $pubPath -Raw).Trim()
        Write-Host "Using manual_public.key"
    }
}

if (-not $pub) {
    Write-Host "No key found. Connect once in MScale app (creates wg_private.key) or run quick-exit-via.ps1" -ForegroundColor Red
    exit 1
}

Write-Host "Applying India exit routing for overlay $overlay ..."
$body = @{ public_key = $pub; overlay_ip = $overlay } | ConvertTo-Json
try {
    $r = Invoke-RestMethod -Uri "http://129.151.146.44:8081/api/exit-route/ensure-by-key" `
        -Method POST -Body $body -ContentType "application/json" -TimeoutSec 20
    Write-Host "OK: client=$($r.client_ip) exit=$($r.exit_ip)" -ForegroundColor Green
    Write-Host "Now Connect in MScale (admin) and check https://ifconfig.me"
    exit 0
} catch {
    Write-Host "API failed: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "Try: ssh ph and run ops/recover-ph-ssh.sh" -ForegroundColor Yellow
    exit 1
}
