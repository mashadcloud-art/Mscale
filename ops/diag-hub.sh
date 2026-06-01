#!/bin/bash
DB=/home/ubuntu/mscale-server/mscale.db
echo "=== online/recent devices ==="
sqlite3 "$DB" "SELECT u.email, d.overlay_ip, d.tunnel_mode, substr(d.exit_node_id,1,40), d.status FROM devices d JOIN users u ON u.id=d.user_id WHERE d.last_seen_at > datetime('now','-30 minutes') ORDER BY d.last_seen_at DESC;"

echo "=== enabled exit nodes ==="
sqlite3 "$DB" "SELECT u.email, d.overlay_ip, d.device_name, en.id FROM exit_nodes en JOIN devices d ON d.id=en.device_id JOIN users u ON u.id=en.owner_user_id WHERE en.is_enabled=1;"

echo "=== wg peers with handshake <5m ==="
sudo wg show wg0 dump | awk -F'\t' 'NR>1 && $5>0 {print $1, $3, $4, $5}'

echo "=== route tests ==="
for ip in 100.64.1.2 100.64.1.3 100.64.1.4 100.64.1.11; do
  echo -n "$ip -> 8.8.8.8: "
  sudo ip route get 8.8.8.8 from $ip iif wg0 2>&1 | head -1
done

echo "=== ip rules table 100 ==="
sudo ip rule list | grep 100
sudo ip route show table 100
