package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

type DeviceUpdateKeyRequest struct {
	DeviceID  string `json:"device_id"`
	PublicKey string `json:"public_key"`
}

type DeviceUpdateKeyResponse struct {
	Message   string `json:"message"`
	DeviceID  string `json:"device_id"`
	PublicKey string `json:"public_key"`
}

func (h *AuthHandler) UpdateDevicePublicKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req DeviceUpdateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.PublicKey = strings.TrimSpace(req.PublicKey)

	if req.DeviceID == "" || req.PublicKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and public_key are required"})
		return
	}

	result, err := h.DB.Exec(
		`UPDATE devices
		 SET public_key = ?
		 WHERE id = ? AND user_id = ?`,
		req.PublicKey, req.DeviceID, session.UserID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update device public key"})
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

	writeJSON(w, http.StatusOK, DeviceUpdateKeyResponse{
		Message:   "device public key updated successfully",
		DeviceID:  req.DeviceID,
		PublicKey: req.PublicKey,
	})
}

