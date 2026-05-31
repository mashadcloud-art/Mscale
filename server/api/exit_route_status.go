package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type exitRouteStatusResponse struct {
	Connected      bool    `json:"connected"`
	TunnelMode     string  `json:"tunnel_mode"`
	ExitNodeID     *string `json:"exit_node_id,omitempty"`
	ExitDeviceName string  `json:"exit_device_name,omitempty"`
	ExitOverlayIP  string  `json:"exit_overlay_ip,omitempty"`
	ExitCountry    string  `json:"exit_country,omitempty"`
	ClientDevice   string  `json:"client_device_name,omitempty"`
	ClientOverlay  string  `json:"client_overlay_ip,omitempty"`
	Message        string  `json:"message"`
}

// ExitRouteStatus reports whether this client is routing via an exit node.
func (h *AuthHandler) ExitRouteStatus(w http.ResponseWriter, r *http.Request) {
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

	out, err := h.lookupExitRouteStatus(deviceID, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *AuthHandler) lookupExitRouteStatus(deviceID, userID string) (exitRouteStatusResponse, error) {
	var deviceName, tunnelMode, overlayIP, exitNodeID sql.NullString
	err := h.DB.QueryRow(
		`SELECT device_name, tunnel_mode, overlay_ip, exit_node_id
		 FROM devices WHERE id = ? AND user_id = ?`,
		deviceID, userID,
	).Scan(&deviceName, &tunnelMode, &overlayIP, &exitNodeID)
	if err == sql.ErrNoRows {
		return exitRouteStatusResponse{}, fmt.Errorf("device not found")
	}
	if err != nil {
		return exitRouteStatusResponse{}, fmt.Errorf("database error")
	}

	out := exitRouteStatusResponse{
		TunnelMode:    strings.TrimSpace(tunnelMode.String),
		ClientDevice:  deviceName.String,
		ClientOverlay: overlayIP.String,
		Message:       "Not using an exit node — mesh only or disconnected.",
	}

	mode := strings.ToLower(out.TunnelMode)
	if mode != "exit-via" || !exitNodeID.Valid || exitNodeID.String == "" {
		return out, nil
	}

	out.Connected = true
	id := exitNodeID.String
	out.ExitNodeID = &id

	if id == "hub" {
		out.ExitDeviceName = "Oracle Cloud Hub"
		out.ExitOverlayIP = "100.64.0.1"
		out.ExitCountry = "IN"
		out.Message = fmt.Sprintf(
			"Confirmed: %s is routing internet via Oracle Cloud Hub (100.64.0.1).",
			out.ClientDevice,
		)
		return out, nil
	}

	var exitName, exitOverlay, country sql.NullString
	err = h.DB.QueryRow(`
		SELECT d.device_name, d.overlay_ip, en.country_code
		FROM exit_nodes en
		INNER JOIN devices d ON d.id = en.device_id
		WHERE en.id = ? AND en.owner_user_id = ?
	`, id, userID).Scan(&exitName, &exitOverlay, &country)
	if err != nil {
		out.Message = "Exit route is set but exit device details could not be loaded."
		return out, nil
	}

	out.ExitDeviceName = exitName.String
	out.ExitOverlayIP = exitOverlay.String
	out.ExitCountry = country.String
	if out.ExitOverlayIP == "" {
		out.Message = fmt.Sprintf(
			"Exit node %s is registered but not online on the mesh yet.",
			out.ExitDeviceName,
		)
		return out, nil
	}

	cc := out.ExitCountry
	if cc != "" {
		cc = " (" + cc + ")"
	}
	out.Message = fmt.Sprintf(
		"Confirmed: %s is routing internet via %s%s at VPN IP %s.",
		out.ClientDevice, out.ExitDeviceName, cc, out.ExitOverlayIP,
	)
	return out, nil
}

type testExitRouteRequest struct {
	DeviceID string `json:"device_id"`
}

// TestExitRoute sends a test notification to the active exit device for this client.
func (h *AuthHandler) TestExitRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req testExitRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id is required"})
		return
	}

	status, err := h.lookupExitRouteStatus(req.DeviceID, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if !status.Connected || status.ExitNodeID == nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "not routing via exit node",
			"message": "Connect in Exit Node mode and pick an exit device first.",
		})
		return
	}

	exitNodeID := *status.ExitNodeID
	if exitNodeID == "hub" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message":       status.Message,
			"notification":  "Hub exit — no device notification (traffic uses Oracle hub).",
			"exit_device":   status.ExitDeviceName,
			"exit_overlay":  status.ExitOverlayIP,
			"client_device": status.ClientDevice,
		})
		return
	}

	var exitDeviceID string
	err = h.DB.QueryRow(
		`SELECT device_id FROM exit_nodes WHERE id = ? AND owner_user_id = ?`,
		exitNodeID, session.UserID,
	).Scan(&exitDeviceID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "exit device not found"})
		return
	}

	msg := fmt.Sprintf("%s is using you as exit node (test ping)", status.ClientDevice)
	payload := map[string]interface{}{
		"from_device_id":   req.DeviceID,
		"from_device_name": status.ClientDevice,
		"message":          msg,
		"sent_at":          time.Now().UTC().Format(time.RFC3339),
	}
	cmdID, err := h.insertDeviceCommand(exitDeviceID, session.UserID, "exit_test_ping", payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not queue test notification"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":          status.Message,
		"notification":     fmt.Sprintf("Test sent to %s — check that device for a popup.", status.ExitDeviceName),
		"command_id":       cmdID,
		"exit_device":      status.ExitDeviceName,
		"exit_device_id":   exitDeviceID,
		"exit_overlay":     status.ExitOverlayIP,
		"client_device":    status.ClientDevice,
		"client_overlay":   status.ClientOverlay,
	})
}
