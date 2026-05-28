package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"
)

var (
	exitRouteMu         sync.Mutex
	activeExitPeer      string // wg public key (base64) for current exit node peer
	activeExitOverlay   string // overlay IP of exit node (India)
	activeExitClientIP  string // overlay IP of client using exit-via
)

type ActivateExitRouteRequest struct {
	DeviceID   string `json:"device_id"`
	ExitNodeID string `json:"exit_node_id"`
}

// ActivateExitRoute tells the hub (wg0) to send internet-bound traffic via the exit device peer.
func (h *AuthHandler) ActivateExitRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req ActivateExitRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.ExitNodeID = strings.TrimSpace(req.ExitNodeID)
	if req.DeviceID == "" || req.ExitNodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id and exit_node_id are required"})
		return
	}

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
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}
	if err != nil || clientOverlay == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "client device not enrolled on mesh"})
		return
	}

	exitB64, err := normalizeWGPublicKey(exitPubKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid exit node public key"})
		return
	}

	exitRouteMu.Lock()
	defer exitRouteMu.Unlock()

	// Bug 1: exit peer needs /32 + 0.0.0.0/1 + 128.0.0.0/1 (not /32 alone).
	if err := syncExitNodePeer(exitOverlay, exitPubKey); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "could not configure exit peer on hub",
			"details": err.Error(),
		})
		return
	}
	if err := syncWGPeer(clientOverlay, clientPubKey, ""); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "could not configure client peer on hub",
			"details": err.Error(),
		})
		return
	}
	if err := ensureHubExitForwarding(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "hub forwarding setup failed",
			"details": err.Error(),
		})
		return
	}
	if activeExitClientIP != "" && activeExitClientIP != clientOverlay {
		clearClientExitPolicy(activeExitClientIP)
	}
	if err := ensureClientExitPolicy(clientOverlay); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "hub exit policy failed (sudo ip rule/route on ph)",
			"details": err.Error(),
		})
		return
	}
	if err := ensureHubExitNAT(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "hub NAT setup failed",
			"details": err.Error(),
		})
		return
	}

	activeExitPeer = exitB64
	activeExitOverlay = exitOverlay
	activeExitClientIP = clientOverlay

	_, _ = h.DB.Exec(
		`UPDATE devices SET tunnel_mode = 'exit-via', exit_node_id = ? WHERE id = ? AND user_id = ?`,
		req.ExitNodeID, req.DeviceID, session.UserID,
	)
	NotifyDevicesChanged()

	writeJSON(w, http.StatusOK, map[string]string{
		"message":     "exit routing active",
		"exit_ip":     exitOverlay,
		"client_ip":   clientOverlay,
	})
}

// reapplyExitRouteForDevice restores hub exit policy after reconnect (enroll path).
func (h *AuthHandler) reapplyExitRouteForDevice(deviceID, exitNodeID, userID string) error {
	exitOverlay, exitPubKey, err := h.lookupExitPeerKeys(exitNodeID, userID)
	if err != nil {
		return err
	}
	var clientOverlay, clientPubKey string
	err = h.DB.QueryRow(
		`SELECT overlay_ip, public_key FROM devices WHERE id = ? AND user_id = ?`,
		deviceID, userID,
	).Scan(&clientOverlay, &clientPubKey)
	if err != nil || clientOverlay == "" {
		return fmt.Errorf("client not enrolled")
	}
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
	if activeExitClientIP != "" && activeExitClientIP != clientOverlay {
		clearClientExitPolicy(activeExitClientIP)
	}
	if err := ensureClientExitPolicy(clientOverlay); err != nil {
		return err
	}
	_ = ensureHubExitNAT()
	activeExitPeer = exitB64
	activeExitOverlay = exitOverlay
	activeExitClientIP = clientOverlay
	return nil
}

type DeactivateExitRouteRequest struct {
	DeviceID string `json:"device_id"`
}

func (h *AuthHandler) DeactivateExitRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req DeactivateExitRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.DeviceID = strings.TrimSpace(req.DeviceID)

	exitRouteMu.Lock()
	defer exitRouteMu.Unlock()

	// Bug 2 cleanup only: remove client policy rule. Keep India /1 routes on exit peer.
	if activeExitClientIP != "" {
		clearClientExitPolicy(activeExitClientIP)
		activeExitClientIP = ""
	}

	if req.DeviceID != "" {
		_, _ = h.DB.Exec(
			`UPDATE devices SET tunnel_mode = 'mesh' WHERE id = ? AND user_id = ?`,
			req.DeviceID, session.UserID,
		)
	}
	NotifyDevicesChanged()
	writeJSON(w, http.StatusOK, map[string]string{"message": "exit routing deactivated"})
}

// ensureHubExitForwarding allows WireGuard peers on wg0 to forward traffic (client -> exit node).
func ensureHubExitForwarding() error {
	rules := [][]string{
		{"-C", "FORWARD", "-i", "wg0", "-o", "wg0", "-j", "ACCEPT"},
		{"-A", "FORWARD", "-i", "wg0", "-o", "wg0", "-j", "ACCEPT"},
		{"-C", "FORWARD", "-i", "wg0", "-o", "wg0", "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
		{"-A", "FORWARD", "-i", "wg0", "-o", "wg0", "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
	}
	for i := 0; i < len(rules); i += 2 {
		check := append([]string{"iptables"}, rules[i]...)
		if exec.Command("sudo", check...).Run() == nil {
			continue
		}
		add := append([]string{"iptables"}, rules[i+1]...)
		if _, err := runSudo(add...); err != nil {
			return err
		}
	}
	if _, err := runSudo("sysctl", "-w", "net.ipv4.ip_forward=1"); err != nil {
		return err
	}
	// Strict rp_filter drops forwarded VPN traffic asymmetrically.
	_, _ = runSudo("sysctl", "-w", "net.ipv4.conf.all.rp_filter=2")
	_, _ = runSudo("sysctl", "-w", "net.ipv4.conf.wg0.rp_filter=2")
	return nil
}

func (h *AuthHandler) lookupExitPeerKeys(exitNodeID, userID string) (overlay, pubKey string, err error) {
	err = h.DB.QueryRow(`
		SELECT d.overlay_ip, d.public_key
		FROM exit_nodes en
		INNER JOIN devices d ON d.id = en.device_id
		WHERE en.id = ? AND en.is_enabled = 1
		  AND (en.is_private = 0 OR en.owner_user_id = ? OR EXISTS (
		    SELECT 1 FROM exit_node_access ea
		    WHERE ea.exit_node_id = en.id AND ea.grantee_user_id = ? AND ea.status = 'active'
		  ))
	`, exitNodeID, userID, userID).Scan(&overlay, &pubKey)
	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("exit node not found or not allowed")
	}
	if err != nil {
		return "", "", fmt.Errorf("database error")
	}
	if overlay == "" {
		return "", "", fmt.Errorf("exit node is not online on the mesh")
	}
	return overlay, pubKey, nil
}
