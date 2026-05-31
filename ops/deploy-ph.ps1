# Deploy mscale-server to UAE hub (SSH host: ph, ARM64 Ubuntu).
$ErrorActionPreference = "Stop"
$ssh = "$env:WINDIR\System32\OpenSSH\ssh.exe"
$scp = "$env:WINDIR\System32\OpenSSH\scp.exe"
if (-not (Test-Path $ssh)) { throw "OpenSSH not found. Install OpenSSH Client in Windows Optional Features." }

$serverDir = Join-Path $PSScriptRoot "..\server"
Write-Host "Building linux/arm64 mscale-server..."
Push-Location $serverDir
$env:GOOS = "linux"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "0"
go build -o mscale-server .
Pop-Location
$bin = Join-Path $serverDir "mscale-server"
if (-not (Test-Path $bin)) { throw "build failed" }

Write-Host "Uploading to ph..."
& $scp $bin ph:/tmp/mscale-server
& $scp (Join-Path $PSScriptRoot "sudoers-mscale-hub.example") ph:/tmp/sudoers-mscale-hub.example
& $scp (Join-Path $serverDir "dashboard.html") ph:/tmp/dashboard.html

Write-Host "Installing on ph..."
& $ssh ph "sudo mv /tmp/mscale-server /home/ubuntu/mscale-server/mscale-server && sudo chmod 755 /home/ubuntu/mscale-server/mscale-server"
& $ssh ph "sudo mv /tmp/dashboard.html /home/ubuntu/mscale-server/dashboard.html"
& $ssh ph "test -f /etc/sudoers.d/mscale-hub || (sudo cp /tmp/sudoers-mscale-hub.example /etc/sudoers.d/mscale-hub && sudo chmod 440 /etc/sudoers.d/mscale-hub && sudo visudo -cf /etc/sudoers.d/mscale-hub)"
& $ssh ph "pm2 restart mscale-server 2>/dev/null || sudo systemctl restart mscale-server"
& $ssh ph "curl -s -o /dev/null -w 'API HTTP %{http_code}' http://127.0.0.1:8081/api/me; echo"

Write-Host "Done. If activate fails in app, error details now come from hub policy verify."
