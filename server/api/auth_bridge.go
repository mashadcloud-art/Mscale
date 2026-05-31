package api

import (
	"database/sql"
	"net/http"
	"strings"
	"time"
)

// CreateBridgeToken issues a one-time token so the desktop app can open the web admin already signed in.
func (h *AuthHandler) CreateBridgeToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	token, err := generateSessionID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create bridge token"})
		return
	}

	expiresAt := time.Now().Add(3 * time.Minute)
	_, err = h.DB.Exec(
		`INSERT INTO auth_bridge_tokens (token, session_id, expires_at) VALUES (?, ?, ?)`,
		token, session.ID, expiresAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not store bridge token"})
		return
	}

	bridgePath := "/mscale/api/auth/bridge?token=" + token + "&desktop=1"
	writeJSON(w, http.StatusOK, map[string]string{
		"bridge_path": bridgePath,
		"bridge_url":  "https://mashad.shop" + bridgePath,
	})
}

// BridgeLogin consumes a one-time token and sets the session cookie for the web admin.
func (h *AuthHandler) BridgeLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token required"})
		return
	}

	var sessionID string
	var expiresAt time.Time
	err := h.DB.QueryRow(
		`SELECT session_id, expires_at FROM auth_bridge_tokens WHERE token = ?`,
		token,
	).Scan(&sessionID, &expiresAt)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired bridge token"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	_, _ = h.DB.Exec(`DELETE FROM auth_bridge_tokens WHERE token = ?`, token)

	if time.Now().After(expiresAt) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "bridge token expired"})
		return
	}

	var userExpires time.Time
	err = h.DB.QueryRow(
		`SELECT expires_at FROM user_sessions WHERE id = ?`,
		sessionID,
	).Scan(&userExpires)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session no longer valid"})
		return
	}

	h.writeSessionCookie(w, r, sessionID, userExpires)

	redirect := "/mscale/?desktop=1"
	if strings.TrimSpace(r.URL.Query().Get("desktop")) != "1" {
		redirect = "/mscale/"
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}
