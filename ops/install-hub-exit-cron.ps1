# One-time: install persistent exit routing on ph (runs every minute).
$ErrorActionPreference = "Stop"
$ssh = "$env:WINDIR\System32\OpenSSH\ssh.exe"
$scp = "$env:WINDIR\System32\OpenSSH\scp.exe"
$script = Join-Path $PSScriptRoot "ensure-exit-routing.sh"

& $scp $script ph:/tmp/ensure-exit-routing.sh
& $ssh ph @'
sudo cp /tmp/ensure-exit-routing.sh /usr/local/bin/mscale-ensure-exit-routing.sh
sudo chmod 755 /usr/local/bin/mscale-ensure-exit-routing.sh
echo '* * * * * root /usr/local/bin/mscale-ensure-exit-routing.sh' | sudo tee /etc/cron.d/mscale-exit-routing
sudo chmod 644 /etc/cron.d/mscale-exit-routing
sudo /usr/local/bin/mscale-ensure-exit-routing.sh
echo installed
'@

Write-Host "Hub will re-apply India exit routing every minute."
