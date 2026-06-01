package api

import (
	"database/sql"
	"log"
)

// RestoreHubExitRoutes re-applies exit routing after server restart (in-memory state was lost).
func RestoreHubExitRoutes(db *sql.DB) {
	pruneWGZombiePeers()

	rows, err := db.Query(`
		SELECT d.overlay_ip, d.public_key
		FROM exit_nodes en
		INNER JOIN devices d ON d.id = en.device_id
		WHERE en.is_enabled = 1 AND d.overlay_ip IS NOT NULL AND d.overlay_ip != ''
	`)
	if err != nil {
		log.Printf("WARN: restore exit nodes: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var overlay, pub string
		if err := rows.Scan(&overlay, &pub); err != nil {
			continue
		}
		if err := syncExitNodePeer(overlay, pub); err != nil {
			log.Printf("WARN: restore exit peer %s: %v", overlay, err)
		} else {
			log.Printf("INFO: exit peer %s internet routes restored", overlay)
		}
	}

	clientRows, err := db.Query(`
		SELECT c.overlay_ip, x.overlay_ip
		FROM devices c
		INNER JOIN exit_nodes en ON en.id = c.exit_node_id
		INNER JOIN devices x ON x.id = en.device_id
		WHERE c.tunnel_mode = 'exit-via'
		  AND c.overlay_ip IS NOT NULL AND c.overlay_ip != ''
		  AND x.overlay_ip IS NOT NULL AND x.overlay_ip != ''
	`)
	if err != nil {
		return
	}
	defer clientRows.Close()
	for clientRows.Next() {
		var clientOverlay, exitOverlay string
		if err := clientRows.Scan(&clientOverlay, &exitOverlay); err != nil {
			continue
		}
		if err := ensureClientExitPolicy(clientOverlay, exitOverlay); err != nil {
			log.Printf("WARN: restore client policy %s via %s: %v", clientOverlay, exitOverlay, err)
		} else if err := ensureMobileExitPath(); err != nil {
			log.Printf("WARN: restore mobile exit path: %v", err)
		} else {
			log.Printf("INFO: client exit policy restored for %s via %s", clientOverlay, exitOverlay)
		}
	}
}
