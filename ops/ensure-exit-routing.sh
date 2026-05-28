#!/bin/bash
# Idempotent: keep India exit + client policy routing on UAE hub.
# Install: sudo cp ensure-exit-routing.sh /usr/local/bin/mscale-ensure-exit-routing.sh
#          sudo chmod 755 /usr/local/bin/mscale-ensure-exit-routing.sh
# Cron:    * * * * * root /usr/local/bin/mscale-ensure-exit-routing.sh

set -e
CLIENT_IP="${MSCALE_CLIENT_IP:-100.64.1.11}"
EXIT_KEY="${MSCALE_EXIT_PEER:-dImB48LL2IjKtPhJYlITFmYihA+6wZRVC71Zg1CRKh8=}"
EXIT_IP="${MSCALE_EXIT_IP:-100.64.1.2}"
TABLE=100

command -v wg >/dev/null || exit 0
wg show wg0 >/dev/null 2>&1 || exit 0

# Bug 1: India peer must carry split routes for internet.
wg set wg0 peer "$EXIT_KEY" \
  allowed-ips "${EXIT_IP}/32,0.0.0.0/1,128.0.0.0/1" \
  persistent-keepalive 25 2>/dev/null || true

# Register manual desktop peer if present in env.
if [ -n "${MSCALE_CLIENT_PUBKEY:-}" ]; then
  wg set wg0 peer "$MSCALE_CLIENT_PUBKEY" \
    allowed-ips "${CLIENT_IP}/32" persistent-keepalive 25 2>/dev/null || true
fi

# Bug 2: policy route for client overlay.
while ip rule del from "${CLIENT_IP}/32" 2>/dev/null; do :; done
ip rule add pref 100 from "${CLIENT_IP}/32" lookup "$TABLE" 2>/dev/null || true
ip route replace 0.0.0.0/1 dev wg0 table "$TABLE" 2>/dev/null || true
ip route replace 128.0.0.0/1 dev wg0 table "$TABLE" 2>/dev/null || true

# Forward + NAT
iptables -t nat -C POSTROUTING -o wg0 -j MASQUERADE 2>/dev/null || \
  iptables -t nat -A POSTROUTING -o wg0 -j MASQUERADE 2>/dev/null || true
iptables -C FORWARD -i wg0 -o wg0 -j ACCEPT 2>/dev/null || \
  iptables -A FORWARD -i wg0 -o wg0 -j ACCEPT 2>/dev/null || true
sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1 || true
sysctl -w net.ipv4.conf.all.rp_filter=2 >/dev/null 2>&1 || true
