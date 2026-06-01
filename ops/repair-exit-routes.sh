#!/bin/bash
# Re-apply exit routing for all exit-via clients on the UAE hub.
set -e
DB=/home/ubuntu/mscale-server/mscale.db
TABLE=100

echo "Pruning zombie wg peers..."
sudo wg show wg0 dump | awk -F'\t' 'NR>1 && $4=="(none)" {print $1}' | while read -r pub; do
  [ -n "$pub" ] && sudo wg set wg0 peer "$pub" remove 2>/dev/null || true
done

echo "Syncing enabled exit providers (internet routes)..."
sqlite3 "$DB" "SELECT d.overlay_ip, d.public_key FROM exit_nodes en JOIN devices d ON d.id=en.device_id WHERE en.is_enabled=1 AND d.overlay_ip IS NOT NULL AND d.overlay_ip != '';" | while IFS='|' read -r oip pkey; do
  [ -z "$oip" ] && continue
  b64=$(echo "$pkey" | tr -d '[:space:]')
  if [ ${#b64} -eq 64 ]; then
    raw=$(echo "$b64" | xxd -r -p 2>/dev/null || true)
    [ -n "$raw" ] && b64=$(echo -n "$raw" | base64 -w0)
  fi
  [ -z "$b64" ] && continue
  echo "  exit peer $oip"
  sudo wg set wg0 peer "$b64" allowed-ips "${oip}/32,0.0.0.0/1,128.0.0.0/1" persistent-keepalive 25 2>/dev/null || true
done

echo "Applying client exit policies..."
sqlite3 "$DB" "SELECT c.overlay_ip, x.overlay_ip FROM devices c INNER JOIN exit_nodes en ON en.id=c.exit_node_id INNER JOIN devices x ON x.id=en.device_id WHERE c.tunnel_mode='exit-via' AND c.overlay_ip IS NOT NULL AND c.overlay_ip != '' AND x.overlay_ip IS NOT NULL AND x.overlay_ip != '';" | while IFS='|' read -r client exit; do
  [ -z "$client" ] || [ -z "$exit" ] && continue
  echo "  client $client via exit $exit"
  while sudo ip rule del from "${client}/32" 2>/dev/null; do :; done
  sudo ip rule add pref 100 from "${client}/32" lookup "$TABLE" 2>/dev/null || true
  sudo ip route replace 0.0.0.0/1 dev wg0 table "$TABLE" 2>/dev/null || true
  sudo ip route replace 128.0.0.0/1 dev wg0 table "$TABLE" 2>/dev/null || true
  sudo ip route get 8.8.8.8 from "$client" iif wg0 2>&1 | head -1
done

sudo iptables -C FORWARD -i wg0 -o wg0 -j ACCEPT 2>/dev/null || sudo iptables -A FORWARD -i wg0 -o wg0 -j ACCEPT
sudo sysctl -w net.ipv4.ip_forward=1 >/dev/null
echo "Done."
