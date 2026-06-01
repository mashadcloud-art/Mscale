package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type WakeDeviceRequest struct {
	TargetDeviceID string `json:"target_device_id"`
	Action         string `json:"action,omitempty"` // connect, enable_exit, route_via, mesh
	ExitNodeID     string `json:"exit_node_id,omitempty"`
	CountryCode    string `json:"country_code,omitempty"`
	Label          string `json:"label,omitempty"`
}

type WakeConfigRequest struct {
	DeviceID    string   `json:"device_id"`
	Enabled     bool     `json:"enabled"`
	WakeNumbers []string `json:"wake_numbers,omitempty"`
	DevicePhone string   `json:"device_phone,omitempty"`
}

type CommandAckRequest struct {
	CommandID string `json:"command_id"`
	DeviceID  string `json:"device_id"`
	Status    string `json:"status,omitempty"` // completed, failed
}

type PendingCommandResponse struct {
	CommandID   string                 `json:"command_id"`
	CommandType string                 `json:"command_type"`
	Payload     map[string]interface{} `json:"payload"`
}

func makeCommandID(deviceID string) string {
	return fmt.Sprintf("cmd-%s-%s", deviceID, time.Now().Format("20060102150405.000"))
}

func (h *AuthHandler) ensureDeviceOwned(deviceID, userID string) error {
	var id string
	err := h.DB.QueryRow(
		`SELECT id FROM devices WHERE id = ? AND user_id = ?`,
		deviceID, userID,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return fmt.Errorf("device not found")
	}
	return err
}

func (h *AuthHandler) insertDeviceCommand(deviceID, userID, commandType string, payload map[string]interface{}) (string, error) {
	payloadJSON, _ := json.Marshal(payload)
	cmdID := makeCommandID(deviceID)
	expires := time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339)
	_, err := h.DB.Exec(
		`INSERT INTO device_commands (id, device_id, user_id, command_type, payload_json, status, expires_at)
		 VALUES (?, ?, ?, ?, ?, 'pending', ?)`,
		cmdID, deviceID, userID, commandType, string(payloadJSON), expires,
	)
	return cmdID, err
}

// WakeDevice queues a remote wake/connect command for a target device.
func (h *AuthHandler) WakeDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req WakeDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	req.TargetDeviceID = strings.TrimSpace(req.TargetDeviceID)
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	if req.Action == "" {
		req.Action = "connect"
	}
	req.ExitNodeID = strings.TrimSpace(req.ExitNodeID)
	req.CountryCode = strings.ToUpper(strings.TrimSpace(req.CountryCode))
	req.Label = strings.TrimSpace(req.Label)

	if req.TargetDeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target_device_id is required"})
		return
	}
	if err := h.ensureDeviceOwned(req.TargetDeviceID, session.UserID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	payload := map[string]interface{}{
		"action": req.Action,
	}
	if req.ExitNodeID != "" {
		payload["exit_node_id"] = req.ExitNodeID
	}
	if req.CountryCode != "" {
		payload["country_code"] = req.CountryCode
	}
	if req.Label != "" {
		payload["label"] = req.Label
	}

	cmdID, err := h.insertDeviceCommand(req.TargetDeviceID, session.UserID, "wake_connect", payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not queue wake command"})
		return
	}

	var wakeEnabled int
	var wakeNumbers sql.NullString
	_ = h.DB.QueryRow(
		`SELECT wake_remote_enabled, wake_call_numbers FROM devices WHERE id = ?`,
		req.TargetDeviceID,
	).Scan(&wakeEnabled, &wakeNumbers)

	NotifyDevicesChanged()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":            "wake command queued",
		"command_id":         cmdID,
		"target_device_id":   req.TargetDeviceID,
		"wake_remote_enabled": wakeEnabled == 1,
		"wake_call_numbers":  wakeNumbers.String,
		"hint":               "If Wake on Call is enabled on the device, call it now. Otherwise the app will pick up the command on its next wake poll.",
	})
}

// SetWakeConfig stores call-wake settings on a device (configured from the mobile app or admin).
func (h *AuthHandler) SetWakeConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req WakeConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id is required"})
		return
	}
	if err := h.ensureDeviceOwned(req.DeviceID, session.UserID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	numbers := make([]string, 0, len(req.WakeNumbers))
	for _, n := range req.WakeNumbers {
		n = normalizePhone(n)
		if n != "" {
			numbers = append(numbers, n)
		}
	}
	numbersStr := strings.Join(numbers, ",")
	enabled := 0
	if req.Enabled {
		enabled = 1
	}
	phone := normalizePhone(req.DevicePhone)

	_, err = h.DB.Exec(
		`UPDATE devices SET wake_remote_enabled = ?, wake_call_numbers = ?, device_phone = COALESCE(NULLIF(?, ''), device_phone) WHERE id = ? AND user_id = ?`,
		enabled, numbersStr, phone, req.DeviceID, session.UserID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	NotifyDevicesChanged()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "wake config saved",
		"device_id":     req.DeviceID,
		"enabled":       req.Enabled,
		"wake_numbers":  numbers,
		"device_phone":  phone,
	})
}

// SetDevicePhone stores the phone number admins can dial to wake this device.
func (h *AuthHandler) SetDevicePhone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req struct {
		DeviceID    string `json:"device_id"`
		DevicePhone string `json:"device_phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.DevicePhone = normalizePhone(req.DevicePhone)
	if req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id is required"})
		return
	}
	if err := h.ensureDeviceOwned(req.DeviceID, session.UserID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	_, err = h.DB.Exec(
		`UPDATE devices SET device_phone = ? WHERE id = ? AND user_id = ?`,
		req.DevicePhone, req.DeviceID, session.UserID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	NotifyDevicesChanged()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "phone saved",
		"device_id":    req.DeviceID,
		"device_phone": req.DevicePhone,
	})
}

// GetPendingCommand returns the oldest pending command for a device (polled by mobile/desktop).
func (h *AuthHandler) GetPendingCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	if deviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id is required"})
		return
	}
	if err := h.ensureDeviceOwned(deviceID, session.UserID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	var cmdID, cmdType, payloadJSON string
	err = h.DB.QueryRow(`
		SELECT id, command_type, payload_json
		FROM device_commands
		WHERE device_id = ? AND user_id = ? AND status = 'pending'
		  AND (expires_at IS NULL OR expires_at > datetime('now'))
		ORDER BY created_at ASC
		LIMIT 1
	`, deviceID, session.UserID).Scan(&cmdID, &cmdType, &payloadJSON)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusOK, map[string]interface{}{"command": nil})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	payload := map[string]interface{}{}
	_ = json.Unmarshal([]byte(payloadJSON), &payload)

	_, _ = h.DB.Exec(`UPDATE device_commands SET status = 'delivered' WHERE id = ?`, cmdID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"command": PendingCommandResponse{
			CommandID:   cmdID,
			CommandType: cmdType,
			Payload:     payload,
		},
	})
}

// AckDeviceCommand marks a command completed or failed.
func (h *AuthHandler) AckDeviceCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req CommandAckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	req.CommandID = strings.TrimSpace(req.CommandID)
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.Status == "" {
		req.Status = "completed"
	}
	if req.CommandID == "" || req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "command_id and device_id are required"})
		return
	}
	if err := h.ensureDeviceOwned(req.DeviceID, session.UserID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	_, err = h.DB.Exec(
		`UPDATE device_commands SET status = ? WHERE id = ? AND device_id = ? AND user_id = ?`,
		req.Status, req.CommandID, req.DeviceID, session.UserID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	NotifyDevicesChanged()
	writeJSON(w, http.StatusOK, map[string]string{"message": "acknowledged"})
}

// AdminRouteDevice applies exit routing from the admin panel (client device -> exit node).
func (h *AuthHandler) AdminRouteDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req struct {
		DeviceID   string `json:"device_id"`
		ExitNodeID string `json:"exit_node_id"`
		Mode       string `json:"mode"` // exit-via, mesh, enable_exit
		CountryCode string `json:"country_code,omitempty"`
		Label      string `json:"label,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.ExitNodeID = strings.TrimSpace(req.ExitNodeID)
	req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))
	req.CountryCode = strings.ToUpper(strings.TrimSpace(req.CountryCode))
	req.Label = strings.TrimSpace(req.Label)

	if req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id is required"})
		return
	}
	if err := h.ensureDeviceOwned(req.DeviceID, session.UserID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	switch req.Mode {
	case "enable_exit", "exit-node", "exit_node":
		if req.CountryCode == "" {
			req.CountryCode = "IN"
		}
		if req.Label == "" {
			req.Label = "Exit node"
		}
		var exitNodeID string
		err = h.DB.QueryRow(`SELECT id FROM exit_nodes WHERE device_id = ?`, req.DeviceID).Scan(&exitNodeID)
		if err == sql.ErrNoRows {
			exitNodeID = fmt.Sprintf("exit-%s-%s", req.DeviceID, time.Now().Format("20060102150405"))
			_, err = h.DB.Exec(
				`INSERT INTO exit_nodes (id, device_id, owner_user_id, label, country_code, city, is_private, is_enabled, health_status)
				 VALUES (?, ?, ?, ?, ?, NULL, 1, 1, 'healthy')`,
				exitNodeID, req.DeviceID, session.UserID, req.Label, nullIfEmpty(req.CountryCode),
			)
		} else if err == nil {
			_, err = h.DB.Exec(
				`UPDATE exit_nodes SET label = ?, country_code = ?, is_enabled = 1, health_status = 'healthy'
				 WHERE device_id = ? AND owner_user_id = ?`,
				req.Label, nullIfEmpty(req.CountryCode), req.DeviceID, session.UserID,
			)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not enable exit node"})
			return
		}
		_, _ = h.DB.Exec(`UPDATE devices SET exit_node_id = ?, tunnel_mode = 'exit-node' WHERE id = ? AND user_id = ?`,
			exitNodeID, req.DeviceID, session.UserID)
		_, _ = h.insertDeviceCommand(req.DeviceID, session.UserID, "wake_connect", map[string]interface{}{
			"action":       "enable_exit",
			"country_code": req.CountryCode,
		})
		NotifyDevicesChanged()
		writeJSON(w, http.StatusOK, map[string]string{
			"message":      "device marked as exit node — device must connect and share traffic",
			"exit_node_id": exitNodeID,
		})
		return

	case "disable_exit", "disable-exit":
		_, _ = h.DB.Exec(
			`UPDATE exit_nodes SET is_enabled = 0, health_status = 'disabled' WHERE device_id = ? AND owner_user_id = ?`,
			req.DeviceID, session.UserID,
		)
		_, _ = h.DB.Exec(`UPDATE devices SET tunnel_mode = 'mesh' WHERE id = ? AND user_id = ?`, req.DeviceID, session.UserID)
		NotifyDevicesChanged()
		writeJSON(w, http.StatusOK, map[string]string{"message": "exit node disabled"})
		return

	case "mesh", "":
		exitRouteMu.Lock()
		var clientOverlay sql.NullString
		_ = h.DB.QueryRow(`SELECT overlay_ip FROM devices WHERE id = ? AND user_id = ?`, req.DeviceID, session.UserID).Scan(&clientOverlay)
		if clientOverlay.Valid && clientOverlay.String != "" {
			clearExitClientRouteLocked(clientOverlay.String)
		}
		exitRouteMu.Unlock()
		_, _ = h.DB.Exec(
			`UPDATE devices SET tunnel_mode = 'mesh', exit_node_id = NULL WHERE id = ? AND user_id = ?`,
			req.DeviceID, session.UserID,
		)
		_, _ = h.insertDeviceCommand(req.DeviceID, session.UserID, "wake_connect", map[string]interface{}{
			"action": "mesh",
		})
		NotifyDevicesChanged()
		writeJSON(w, http.StatusOK, map[string]string{"message": "device set to mesh only"})
		return

	case "exit-via", "route_via", "route-via":
		if req.ExitNodeID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "exit_node_id is required for route_via"})
			return
		}
		activateReq := ActivateExitRouteRequest{
			DeviceID:   req.DeviceID,
			ExitNodeID: req.ExitNodeID,
		}
		_ = activateReq
		exitOverlay, exitPubKey, err := h.lookupExitPeerKeys(req.ExitNodeID, session.UserID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		var clientOverlay, clientPubKey string
		err = h.DB.QueryRow(
			`SELECT overlay_ip, public_key FROM devices WHERE id = ? AND user_id = ?`,
			req.DeviceID, session.UserID,
		).Scan(&clientOverlay, &clientPubKey)
		if err != nil || clientOverlay == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "client device not enrolled on mesh"})
			return
		}
		exitRouteMu.Lock()
		defer exitRouteMu.Unlock()
		if err := syncExitNodePeer(exitOverlay, exitPubKey); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hub exit peer setup failed", "details": err.Error()})
			return
		}
		if err := syncWGPeer(clientOverlay, clientPubKey, ""); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hub client peer setup failed", "details": err.Error()})
			return
		}
		if err := ensureHubExitForwarding(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hub forwarding failed", "details": err.Error()})
			return
		}
		clearExitClientRouteLocked(clientOverlay)
		if err := ensureClientExitPolicy(clientOverlay, exitOverlay); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hub exit policy failed", "details": err.Error()})
			return
		}
		if err := ensureMobileExitPath(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hub forwarding failed", "details": err.Error()})
			return
		}
		exitB64, _ := normalizeWGPublicKey(exitPubKey)
		setActiveExitClientLocked(clientOverlay, exitB64, exitOverlay)
		_, _ = h.DB.Exec(
			`UPDATE devices SET tunnel_mode = 'exit-via', exit_node_id = ? WHERE id = ? AND user_id = ?`,
			req.ExitNodeID, req.DeviceID, session.UserID,
		)
		_, _ = h.insertDeviceCommand(req.DeviceID, session.UserID, "wake_connect", map[string]interface{}{
			"action":       "route_via",
			"exit_node_id": req.ExitNodeID,
		})
		NotifyDevicesChanged()
		writeJSON(w, http.StatusOK, map[string]string{
			"message":      "routing updated on hub — device should reconnect to apply",
			"exit_overlay": exitOverlay,
		})
		return

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown mode: " + req.Mode})
	}
}

func normalizePhone(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' || r == '+' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
