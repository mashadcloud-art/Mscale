# Deploy mscale-agent to India VPS (SSH host: pilehead, ARM64 Ubuntu).
$ErrorActionPreference = "Stop"
$ssh = "$env:WINDIR\System32\OpenSSH\ssh.exe"
$scp = "$env:WINDIR\System32\OpenSSH\scp.exe"
$root = $PSScriptRoot

Write-Host "Building linux/arm64 agent..."
Push-Location $root
$env:GOOS = "linux"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "0"
go mod tidy
go build -o mscale-agent .
Pop-Location
if (-not (Test-Path "$root\mscale-agent")) { throw "build failed" }

Write-Host "Uploading to pilehead..."
& $scp "$root\mscale-agent" pilehead:/tmp/mscale-agent
& $scp "$root\mscale-agent.service" pilehead:/tmp/mscale-agent.service
& $scp "$root\agent.env.example" pilehead:/tmp/mscale-agent.env.example

$remote = @"
sudo mkdir -p /usr/local/bin /etc/mscale /var/lib/mscale
sudo mv /tmp/mscale-agent /usr/local/bin/mscale-agent
sudo chmod 755 /usr/local/bin/mscale-agent
sudo mv /tmp/mscale-agent.service /etc/systemd/system/mscale-agent.service
if [ ! -f /etc/mscale/agent.env ]; then sudo cp /tmp/mscale-agent.env.example /etc/mscale/agent.env; sudo chmod 600 /etc/mscale/agent.env; fi
sudo systemctl daemon-reload
sudo systemctl restart mscale-agent
sleep 2
sudo systemctl is-active mscale-agent
"@
& $ssh pilehead $remote

Write-Host "Done. Edit /etc/mscale/agent.env on pilehead if needed."
