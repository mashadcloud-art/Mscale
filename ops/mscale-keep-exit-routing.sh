#!/bin/bash
# Re-apply client exit policy if missing (safe for SSH: Table=off, no 0.0.0.0/0).
CLIENT="${MSCALE_CLIENT_OVERLAY:-100.64.1.11}"
TABLE=100
if ip rule list | grep -q "from ${CLIENT}/32"; then
  exit 0
fi
ip rule add pref 100 from "${CLIENT}/32" lookup "$TABLE" 2>/dev/null || true
ip route replace 0.0.0.0/1 dev wg0 table "$TABLE" 2>/dev/null || true
ip route replace 128.0.0.0/1 dev wg0 table "$TABLE" 2>/dev/null || true
INDIA_KEY="${MSCALE_INDIA_PEER:-dImB48LL2IjKtPhJYlITFmYihA+6wZRVC71Zg1CRKh8=}"
wg set wg0 peer "$INDIA_KEY" \
  allowed-ips 100.64.1.2/32,0.0.0.0/1,128.0.0.0/1 persistent-keepalive 25 2>/dev/null || true
