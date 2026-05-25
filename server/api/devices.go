package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type DeviceRegisterRequest struct {
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	DeviceType string `json:"device_type"`
	PublicKey  string `json:"public_key"`
	AppVersion string `json:"app_version,omitempty"`
	OSVersion  string `json:"os_version,omitempty"`
	CurrentDNS string `json:"current_dns,omitempty"`
	ExitNodeID string `json:"exit_node_id,omitempty"`
	TunnelMode string `json:"tunnel_mode,omitempty"`
	EndpointIP string `json:"endpoint_ip,omitempty"`
}

type DeviceResponse struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	DeviceType string `json:"device_type"`
	PublicKey  string `json:"public_key"`
	AppVersion string `json:"app_version,omitempty"`
	OSVersion  string `json:"os_version,omitempty"`
	CurrentDNS string `json:"current_dns,omitempty"`
	ExitNodeID string `json:"exit_node_id,omitempty"`
	TunnelMode string `json:"tunnel_mode,omitempty"`
	EndpointIP string `json:"endpoint_ip,omitempty"`
	Status     string `json:"status"`
}

func makeDeviceID(userID string) string {
	base := strings.ReplaceAll(userID, " ", "")
	if base == "" {
		base = "device"
	}
	return base + "-dev-" + time.Now().Format("20060102150405")
}

func (h *AuthHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req DeviceRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	req.DeviceName = strings.TrimSpace(req.DeviceName)
	req.Platform = strings.TrimSpace(req.Platform)
	req.DeviceType = strings.TrimSpace(req.DeviceType)
	req.PublicKey = strings.TrimSpace(req.PublicKey)
	req.AppVersion = strings.TrimSpace(req.AppVersion)
	req.OSVersion = strings.TrimSpace(req.OSVersion)
	req.CurrentDNS = strings.TrimSpace(req.CurrentDNS)
	req.ExitNodeID = strings.TrimSpace(req.ExitNodeID)
	req.TunnelMode = strings.TrimSpace(req.TunnelMode)
	req.EndpointIP = strings.TrimSpace(req.EndpointIP)

	if req.DeviceName == "" || req.Platform == "" || req.DeviceType == "" || req.PublicKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_name, platform, device_type, and public_key are required"})
		return
	}

	var existingID string
	err = h.DB.QueryRow(
		"SELECT id FROM devices WHERE public_key = ?",
		req.PublicKey,
	).Scan(&existingID)

	if err != nil && err != sql.ErrNoRows {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if err == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "device already registered"})
		return
	}

	deviceID := makeDeviceID(session.UserID)

	_, err = h.DB.Exec(
		`INSERT INTO devices (
			id, user_id, device_name, platform, device_type, public_key, app_version, os_version, current_dns, exit_node_id, tunnel_mode, endpoint_ip, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		deviceID,
		session.UserID,
		req.DeviceName,
		req.Platform,
		req.DeviceType,
		req.PublicKey,
		req.AppVersion,
		req.OSVersion,
		req.CurrentDNS,
		req.ExitNodeID,
		req.TunnelMode,
		req.EndpointIP,
		"offline",
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not register device"})
		return
	}

	writeJSON(w, http.StatusCreated, DeviceResponse{
		ID:         deviceID,
		UserID:     session.UserID,
		DeviceName: req.DeviceName,
		Platform:   req.Platform,
		DeviceType: req.DeviceType,
		PublicKey:  req.PublicKey,
		AppVersion: req.AppVersion,
		OSVersion:  req.OSVersion,
		CurrentDNS: req.CurrentDNS,
		ExitNodeID: req.ExitNodeID,
		TunnelMode: req.TunnelMode,
		EndpointIP: req.EndpointIP,
		Status:     "offline",
	})
}


