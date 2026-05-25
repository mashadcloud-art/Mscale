package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

type DeviceUpdateMetaRequest struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name,omitempty"`
	Platform   string `json:"platform,omitempty"`
	DeviceType string `json:"device_type,omitempty"`
	AppVersion string `json:"app_version,omitempty"`
	OSVersion  string `json:"os_version,omitempty"`
	CurrentDNS string `json:"current_dns,omitempty"`
	ExitNodeID string `json:"exit_node_id,omitempty"`
	TunnelMode string `json:"tunnel_mode,omitempty"`
	EndpointIP string `json:"endpoint_ip,omitempty"`
	Status     string `json:"status,omitempty"`
}

type DeviceUpdateMetaResponse struct {
	Message string `json:"message"`
	DeviceID string `json:"device_id"`
}

func (h *AuthHandler) UpdateDeviceMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req DeviceUpdateMetaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.DeviceName = strings.TrimSpace(req.DeviceName)
	req.Platform = strings.TrimSpace(req.Platform)
	req.DeviceType = strings.TrimSpace(req.DeviceType)
	req.AppVersion = strings.TrimSpace(req.AppVersion)
	req.OSVersion = strings.TrimSpace(req.OSVersion)
	req.CurrentDNS = strings.TrimSpace(req.CurrentDNS)
	req.ExitNodeID = strings.TrimSpace(req.ExitNodeID)
	req.TunnelMode = strings.TrimSpace(req.TunnelMode)
	req.EndpointIP = strings.TrimSpace(req.EndpointIP)
	req.Status = strings.TrimSpace(req.Status)

	if req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id is required"})
		return
	}

	result, err := h.DB.Exec(
		`UPDATE devices
		 SET device_name = COALESCE(NULLIF(?, ''), device_name),
		     platform = COALESCE(NULLIF(?, ''), platform),
		     device_type = COALESCE(NULLIF(?, ''), device_type),
		     app_version = COALESCE(NULLIF(?, ''), app_version),
		     os_version = COALESCE(NULLIF(?, ''), os_version),
		     current_dns = COALESCE(NULLIF(?, ''), current_dns),
		     exit_node_id = COALESCE(NULLIF(?, ''), exit_node_id),
		     tunnel_mode = COALESCE(NULLIF(?, ''), tunnel_mode),
		     endpoint_ip = COALESCE(NULLIF(?, ''), endpoint_ip),
		     status = COALESCE(NULLIF(?, ''), status),
		     last_seen_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND user_id = ?`,
		req.DeviceName,
		req.Platform,
		req.DeviceType,
		req.AppVersion,
		req.OSVersion,
		req.CurrentDNS,
		req.ExitNodeID,
		req.TunnelMode,
		req.EndpointIP,
		req.Status,
		req.DeviceID,
		session.UserID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update device metadata"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not verify update"})
		return
	}
	if rowsAffected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}

	writeJSON(w, http.StatusOK, DeviceUpdateMetaResponse{
		Message: "device metadata updated successfully",
		DeviceID: req.DeviceID,
	})
}
