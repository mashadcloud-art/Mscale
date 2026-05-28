#!/bin/bash
# Apply SSH-safe hub settings on ph. Idempotent.
set -euo pipefail

echo "=== 1) protect-ssh (before wg-quick) ==="
cat > /tmp/protect-ssh.service <<'UNIT'
[Unit]
Description=Protect SSH before WireGuard
Before=wg-quick@wg0.service
After=network.target

[Service]
Type=oneshot
ExecStart=/sbin/iptables -I INPUT 1 -p tcp --dport 22 -j ACCEPT
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
UNIT
sudo cp /tmp/protect-ssh.service /etc/systemd/system/protect-ssh.service
sudo systemctl daemon-reload
sudo systemctl enable protect-ssh
sudo systemctl start protect-ssh

echo "=== 2) sudoers (LF, hub VPN ops) ==="
cat > /tmp/mscale-hub <<'SUDO'
ubuntu ALL=(root) NOPASSWD: /usr/bin/wg
ubuntu ALL=(root) NOPASSWD: /usr/bin/wg-quick
ubuntu ALL=(root) NOPASSWD: /usr/sbin/ip
ubuntu ALL=(root) NOPASSWD: /usr/sbin/iptables
ubuntu ALL=(root) NOPASSWD: /usr/sbin/sysctl
SUDO
sudo cp /tmp/mscale-hub /etc/sudoers.d/mscale-hub
sudo chmod 440 /etc/sudoers.d/mscale-hub
sudo visudo -cf /etc/sudoers.d/mscale-hub

echo "=== 3) wg0.conf Table=off, no 0.0.0.0/0 ==="
if [ -f /etc/wireguard/wg0.conf ]; then
  sudo cp -a /etc/wireguard/wg0.conf "/etc/wireguard/wg0.conf.bak.$(date +%Y%m%d%H%M%S)"
  sudo sed -i 's/\r$//' /etc/wireguard/wg0.conf
  sudo sed -i 's|0\.0\.0\.0/0|0.0.0.0/1,128.0.0.0/1|g' /etc/wireguard/wg0.conf
  if ! grep -q '^Table = off' /etc/wireguard/wg0.conf; then
    sudo sed -i '/^ListenPort = /a Table = off' /etc/wireguard/wg0.conf
  fi
fi

echo "=== 4) India exit peer routes (runtime) ==="
INDIA_KEY="${MSCALE_INDIA_PEER:-dImB48LL2IjKtPhJYlITFmYihA+6wZRVC71Zg1CRKh8=}"
sudo wg set wg0 peer "$INDIA_KEY" \
  allowed-ips 100.64.1.2/32,0.0.0.0/1,128.0.0.0/1 persistent-keepalive 25 2>/dev/null || true

echo "=== 5) Prune zombie peers (allowed-ips none) ==="
sudo wg show wg0 dump | while IFS=$'\t' read -r pub _ _ _ allowed _; do
  [ -z "${pub:-}" ] && continue
  if [ "$allowed" = "(none)" ] || [ -z "$allowed" ]; then
    sudo wg set wg0 peer "$pub" remove 2>/dev/null || true
  fi
done

echo "=== 6) Restart WireGuard (Table=off applies on restart) ==="
sudo systemctl restart wg-quick@wg0

echo "=== 7) Verify SSH + routing ==="
sudo iptables -C INPUT -p tcp --dport 22 -j ACCEPT
ip route | head -3
sudo wg show wg0 | grep -E 'allowed|interface' | head -8
if sudo wg show wg0 dump | awk -F'\t' '$4 ~ /0\.0\.0\.0\/0/ {found=1} END {exit found?1:0}'; then
  echo "ERROR: still has 0.0.0.0/0 on a peer" >&2
  exit 1
fi
echo "OK: no 0.0.0.0/0 on peers; default route should be enp0s6"
