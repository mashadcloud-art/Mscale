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
		SELECT overlay_ip FROM devices
		WHERE tunnel_mode = 'exit-via' AND overlay_ip IS NOT NULL AND overlay_ip != ''
	`)
	if err != nil {
		return
	}
	defer clientRows.Close()
	for clientRows.Next() {
		var clientOverlay string
		if err := clientRows.Scan(&clientOverlay); err != nil {
			continue
		}
		_ = ensureHubExitForwarding()
		if err := ensureClientExitPolicy(clientOverlay); err != nil {
			log.Printf("WARN: restore client policy %s: %v", clientOverlay, err)
		} else {
			log.Printf("INFO: client exit policy restored for %s", clientOverlay)
		}
	}
}
