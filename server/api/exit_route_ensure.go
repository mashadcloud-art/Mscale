package api

import (
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type EnsureExitByKeyRequest struct {
	PublicKey  string `json:"public_key"`
	OverlayIP  string `json:"overlay_ip,omitempty"`
	ExitNodeID string `json:"exit_node_id,omitempty"`
}

// EnsureExitRouteByKey re-applies hub India exit routing when a client reconnects (no session).
// Used by WireGuard PostUp on Windows. Requires a known device public_key or overlay_ip.
func (h *AuthHandler) EnsureExitRouteByKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req EnsureExitByKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.PublicKey = strings.TrimSpace(req.PublicKey)
	req.OverlayIP = strings.TrimSpace(req.OverlayIP)
	req.ExitNodeID = strings.TrimSpace(req.ExitNodeID)
	if req.PublicKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "public_key is required"})
		return
	}

	clientOverlay := req.OverlayIP
	clientPubKey := req.PublicKey
	var deviceID, userID string
	var exitNodeID sql.NullString

	err := h.DB.QueryRow(
		`SELECT id, user_id, overlay_ip, exit_node_id FROM devices WHERE public_key = ?`,
		req.PublicKey,
	).Scan(&deviceID, &userID, &clientOverlay, &exitNodeID)
	if err == sql.ErrNoRows {
		if b64, e := normalizeWGPublicKey(req.PublicKey); e == nil && b64 != req.PublicKey {
			err = h.DB.QueryRow(
				`SELECT id, user_id, overlay_ip, exit_node_id FROM devices WHERE public_key = ?`,
				b64,
			).Scan(&deviceID, &userID, &clientOverlay, &exitNodeID)
		}
		if err == sql.ErrNoRows {
			if hexKey, e := wgPublicKeyHex(req.PublicKey); e == nil && hexKey != "" {
				err = h.DB.QueryRow(
					`SELECT id, user_id, overlay_ip, exit_node_id FROM devices WHERE public_key = ?`,
					hexKey,
				).Scan(&deviceID, &userID, &clientOverlay, &exitNodeID)
			}
		}
	}
	// Manual WireGuard clients (not in DB): apply routing from request overlay + pubkey.
	if err == sql.ErrNoRows && req.OverlayIP != "" {
		clientOverlay = req.OverlayIP
		clientPubKey = req.PublicKey
		userID = ""
	} else if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found; provide overlay_ip for manual clients"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if clientOverlay == "" {
		clientOverlay = req.OverlayIP
	}
	if clientOverlay == "" {
		clientOverlay = "100.64.1.11"
	}
	if clientPubKey == "" {
		clientPubKey = req.PublicKey
	}
	if req.ExitNodeID != "" {
		exitNodeID = sql.NullString{String: req.ExitNodeID, Valid: true}
	}
	if !exitNodeID.Valid || exitNodeID.String == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "no exit node configured for this device — pick an exit in the app and connect again",
		})
		return
	}

	exitOverlay, exitPubKey, err := h.lookupExitPeerKeys(exitNodeID.String, userID)
	if err != nil && userID == "" {
		exitOverlay, exitPubKey, err = h.lookupExitPeerKeysAny(exitNodeID.String)
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := applyHubExitRouting(clientOverlay, clientPubKey, exitOverlay, exitPubKey); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "could not apply exit routing",
			"details": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message":   "exit routing ensured",
		"client_ip": clientOverlay,
		"exit_ip":   exitOverlay,
	})
}

func applyHubExitRouting(clientOverlay, clientPubKey, exitOverlay, exitPubKey string) error {
	exitB64, err := normalizeWGPublicKey(exitPubKey)
	if err != nil {
		return err
	}
	exitRouteMu.Lock()
	defer exitRouteMu.Unlock()
	if err := syncExitNodePeer(exitOverlay, exitPubKey); err != nil {
		return err
	}
	if err := syncWGPeer(clientOverlay, clientPubKey, ""); err != nil {
		return err
	}
	_ = ensureHubExitForwarding()
	clearExitClientRouteLocked(clientOverlay)
	if err := ensureClientExitPolicy(clientOverlay, exitOverlay); err != nil {
		return err
	}
	if err := ensureMobileExitPath(); err != nil {
		return err
	}
	setActiveExitClientLocked(clientOverlay, exitB64, exitOverlay)
	return nil
}

// wgPublicKeyHex returns the 64-char hex form stored in devices.public_key.
func wgPublicKeyHex(key string) (string, error) {
	b64, err := normalizeWGPublicKey(key)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(raw) != 32 {
		return "", fmt.Errorf("invalid public key")
	}
	return hex.EncodeToString(raw), nil
}

func (h *AuthHandler) lookupExitPeerKeysAny(exitNodeID string) (overlay, pubKey string, err error) {
	err = h.DB.QueryRow(`
		SELECT d.overlay_ip, d.public_key
		FROM exit_nodes en
		INNER JOIN devices d ON d.id = en.device_id
		WHERE en.id = ? AND en.is_enabled = 1
	`, exitNodeID).Scan(&overlay, &pubKey)
	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("exit node not found")
	}
	if overlay == "" {
		return "", "", fmt.Errorf("exit node not on mesh")
	}
	return overlay, pubKey, err
}
