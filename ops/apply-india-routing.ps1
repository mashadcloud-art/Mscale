# Called automatically when WireGuard tunnel goes up (PostUp). No SSH needed.
$ErrorActionPreference = "SilentlyContinue"
$keyDir = Join-Path $env:LOCALAPPDATA "mscale"
$pubPath = Join-Path $keyDir "manual_public.key"
if (-not (Test-Path $pubPath)) { exit 0 }
$pub = (Get-Content $pubPath -Raw).Trim()
$body = @{ public_key = $pub; overlay_ip = "100.64.1.11" } | ConvertTo-Json
try {
    Invoke-RestMethod -Uri "http://129.151.146.44:8081/api/exit-route/ensure-by-key" `
        -Method POST -Body $body -ContentType "application/json" -TimeoutSec 15 | Out-Null
} catch {
    # Hub API unreachable; optional SSH fallback
    $ssh = "$env:WINDIR\System32\OpenSSH\ssh.exe"
    if (Test-Path $ssh) {
        & $ssh -o ConnectTimeout=10 ph "sudo wg set wg0 peer $pub allowed-ips 100.64.1.11/32 persistent-keepalive 25; sudo wg set wg0 peer dImB48LL2IjKtPhJYlITFmYihA+6wZRVC71Zg1CRKh8= allowed-ips 100.64.1.2/32,0.0.0.0/1,128.0.0.0/1 persistent-keepalive 25; sudo ip rule del from 100.64.1.11/32 2>/dev/null; sudo ip rule add pref 100 from 100.64.1.11/32 lookup 100; sudo ip route replace 0.0.0.0/1 dev wg0 table 100; sudo ip route replace 128.0.0.0/1 dev wg0 table 100" 2>$null
    }
}
