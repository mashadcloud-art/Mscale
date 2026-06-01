package api

// syncHubPeerForDevice applies the correct wg0 AllowedIPs for a mesh client or exit provider.
func (h *AuthHandler) syncHubPeerForDevice(deviceID, overlayIP, pubKey string) error {
	if overlayIP == "" || pubKey == "" {
		return nil
	}
	var enabled int
	err := h.DB.QueryRow(
		`SELECT COUNT(*) FROM exit_nodes WHERE device_id = ? AND is_enabled = 1`,
		deviceID,
	).Scan(&enabled)
	if err != nil {
		return syncWGPeer(overlayIP, pubKey, "")
	}
	if enabled > 0 {
		if err := syncExitNodePeer(overlayIP, pubKey); err != nil {
			return err
		}
		h.refreshExitRoutesViaProvider(deviceID)
		return nil
	}
	return syncWGPeer(overlayIP, pubKey, "")
}

// refreshExitRoutesViaProvider re-applies hub policy for clients routing through this exit device.
func (h *AuthHandler) refreshExitRoutesViaProvider(providerDeviceID string) {
	rows, err := h.DB.Query(`
		SELECT d.id, d.exit_node_id, d.user_id
		FROM devices d
		INNER JOIN exit_nodes en ON en.id = d.exit_node_id
		WHERE en.device_id = ?
		  AND d.tunnel_mode = 'exit-via'
		  AND d.exit_node_id IS NOT NULL
		  AND d.exit_node_id != ''`,
		providerDeviceID,
	)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var clientID, exitNodeID, userID string
		if rows.Scan(&clientID, &exitNodeID, &userID) == nil {
			_ = h.reapplyExitRouteForDevice(clientID, exitNodeID, userID)
		}
	}
}
