package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"time"
)

const sessionCookieName = "mscale_session"

type Session struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func sessionCookieSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return false
}

func (h *AuthHandler) writeSessionCookie(w http.ResponseWriter, r *http.Request, sessionID string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   sessionCookieSecure(r),
		Expires:  expires,
	})
}

func (h *AuthHandler) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   sessionCookieSecure(r),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func (h *AuthHandler) CreateSession(w http.ResponseWriter, r *http.Request, userID string) error {
	sessionID, err := generateSessionID()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	_, err = h.DB.Exec(
		"INSERT INTO user_sessions (id, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)",
		sessionID, userID, expiresAt, time.Now(),
	)
	if err != nil {
		return err
	}

	h.writeSessionCookie(w, r, sessionID, expiresAt)

	return nil
}

func (h *AuthHandler) GetSession(r *http.Request) (*Session, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}

	var s Session
	err = h.DB.QueryRow(
		"SELECT id, user_id, expires_at, created_at FROM user_sessions WHERE id = ?",
		cookie.Value,
	).Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}

	if time.Now().After(s.ExpiresAt) {
		_, _ = h.DB.Exec("DELETE FROM user_sessions WHERE id = ?", s.ID)
		return nil, sql.ErrNoRows
	}

	return &s, nil
}

func (h *AuthHandler) ClearSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		_, _ = h.DB.Exec("DELETE FROM user_sessions WHERE id = ?", cookie.Value)
	}

	h.clearSessionCookie(w, r)
}
