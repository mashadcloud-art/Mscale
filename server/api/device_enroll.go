package api

import (
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"
)

type DeviceEnrollRequest struct {
	DeviceID string `json:"device_id"`
}

type DeviceEnrollResponse struct {
	DeviceID   string `json:"device_id"`
	OverlayIP  string `json:"overlay_ip"`
	ServerKey  string `json:"server_key"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	DeviceName string `json:"device_name"`
}

var (
	wgPeerCount = 1
	wgMu        sync.Mutex
)

func getWGServerPublicKey() (string, error) {
	out, err := exec.Command("sudo", "wg", "show", "wg0", "public-key").Output()
	if err != nil {
		return "", fmt.Errorf("failed to read server public key: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func normalizeWGPublicKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("empty public key")
	}

	if raw, err := base64.StdEncoding.DecodeString(key); err == nil && len(raw) == 32 {
		return base64.StdEncoding.EncodeToString(raw), nil
	}

	if raw, err := hex.DecodeString(key); err == nil && len(raw) == 32 {
		return base64.StdEncoding.EncodeToString(raw), nil
	}

	return "", fmt.Errorf("invalid WireGuard public key format")
}

func nextOverlayIP() string {
	wgMu.Lock()
	defer wgMu.Unlock()
	wgPeerCount++
	return fmt.Sprintf("100.64.1.%d", wgPeerCount)
}

func (h *AuthHandler) EnrollDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req DeviceEnrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id is required"})
		return
	}

	var deviceID, userID, deviceName, publicKey string
	var currentOverlayIP sql.NullString

	err = h.DB.QueryRow(
		`SELECT id, user_id, device_name, public_key, overlay_ip
		 FROM devices
		 WHERE id = ? AND user_id = ?`,
		req.DeviceID, session.UserID,
	).Scan(&deviceID, &userID, &deviceName, &publicKey, &currentOverlayIP)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	if currentOverlayIP.Valid && currentOverlayIP.String != "" {
		b64Key, err := normalizeWGPublicKey(publicKey)
		if err == nil {
			exec.Command("sudo", "wg", "set", "wg0", "peer", b64Key, "allowed-ips", currentOverlayIP.String+"/32").Run()
			exec.Command("sudo", "wg-quick", "save", "wg0").Run()
		}

		serverKey, err := getWGServerPublicKey()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not get server key"})
			return
		}

		_, _ = h.DB.Exec(
			`UPDATE devices
			 SET status = ?, last_seen_at = CURRENT_TIMESTAMP
			 WHERE id = ? AND user_id = ?`,
			"online", deviceID, session.UserID,
		)

		writeJSON(w, http.StatusOK, DeviceEnrollResponse{
			DeviceID:   deviceID,
			OverlayIP:  currentOverlayIP.String,
			ServerKey:  serverKey,
			Status:     "online",
			Message:    "device already enrolled",
			DeviceName: deviceName,
		})
		return
	}

	b64Key, err := normalizeWGPublicKey(publicKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "stored public_key is invalid"})
		return
	}

	overlayIP := nextOverlayIP()

	cmd := exec.Command("sudo", "wg", "set", "wg0", "peer", b64Key, "allowed-ips", overlayIP+"/32")
	if out, err := cmd.CombinedOutput(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "could not add peer to wg0",
			"details": string(out),
		})
		return
	}

	_, _ = exec.Command("sudo", "wg-quick", "save", "wg0").CombinedOutput()

	_, err = h.DB.Exec(
		`UPDATE devices
		 SET overlay_ip = ?, status = ?, last_seen_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND user_id = ?`,
		overlayIP, "online", deviceID, session.UserID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update device after enrollment"})
		return
	}

	serverKey, err := getWGServerPublicKey()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not get server key"})
		return
	}

	writeJSON(w, http.StatusOK, DeviceEnrollResponse{
		DeviceID:   deviceID,
		OverlayIP:  overlayIP,
		ServerKey:  serverKey,
		Status:     "online",
		Message:    "device enrolled successfully",
		DeviceName: deviceName,
	})
}
