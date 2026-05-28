#!/bin/bash
# Run on PH-UAE via Oracle "Console connection" if SSH from internet fails.
# Fixes the mistake: India peer (or any peer) must NOT have 0.0.0.0/0 on the hub.

set -e
echo "=== Before ==="
ip route | head -5
sudo wg show wg0 2>/dev/null | head -30 || true
grep -E 'AllowedIPs|0\.0\.0\.0' /etc/wireguard/wg0.conf 2>/dev/null || true

# Stop tunnel so hub uses normal NIC for SSH while we edit
sudo systemctl stop wg-quick@wg0 2>/dev/null || sudo wg-quick down wg0 2>/dev/null || true

# Remove 0.0.0.0/0 from persisted config (main lockout cause)
if [ -f /etc/wireguard/wg0.conf ]; then
  sudo cp -a /etc/wireguard/wg0.conf "/etc/wireguard/wg0.conf.bak.$(date +%Y%m%d%H%M%S)"
  sudo sed -i 's|0\.0\.0\.0/0|0.0.0.0/1,128.0.0.0/1|g' /etc/wireguard/wg0.conf
  sudo sed -i 's|,0\.0\.0\.0/1,128\.0\.0\.0/1,0\.0\.0\.0/1,128\.0\.0\.0/1|,0.0.0.0/1,128.0.0.0/1|g' /etc/wireguard/wg0.conf
fi

# Ensure SSH is allowed before bringing wg back
sudo iptables -C INPUT -p tcp --dport 22 -j ACCEPT 2>/dev/null || \
  sudo iptables -I INPUT 1 -p tcp --dport 22 -j ACCEPT

sudo systemctl unmask wg-quick@wg0 2>/dev/null || true
sudo systemctl start wg-quick@wg0 2>/dev/null || sudo wg-quick up wg0

# India exit peer (adjust pubkey if yours differs)
INDIA_KEY="${MSCALE_INDIA_PEER:-dImB48LL2IjKtPhJYlITFmYihA+6wZRVC71Zg1CRKh8=}"
sudo wg set wg0 peer "$INDIA_KEY" \
  allowed-ips 100.64.1.2/32,0.0.0.0/1,128.0.0.0/1 persistent-keepalive 25 2>/dev/null || true

# Client policy for manual test IP (optional)
CLIENT_IP="${MSCALE_CLIENT_OVERLAY:-100.64.1.11}"
sudo ip rule del from "${CLIENT_IP}/32" 2>/dev/null || true
sudo ip rule add pref 100 from "${CLIENT_IP}/32" lookup 100 2>/dev/null || true
sudo ip route replace 0.0.0.0/1 dev wg0 table 100 2>/dev/null || true
sudo ip route replace 128.0.0.0/1 dev wg0 table 100 2>/dev/null || true

echo "=== After ==="
ip route | head -5
sudo wg show wg0 | grep -E 'allowed|peer' | head -20
echo "DO NOT run: wg-quick save  (until you verify no 0.0.0.0/0 in wg show)"
echo "Test SSH from your PC: ssh ph"
