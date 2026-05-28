package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"mscale-server/utils"
)

func googleClientID() string {
	return strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_ID"))
}

func googleClientSecret() string {
	return strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"))
}

func googleRedirectURI() string {
	if v := strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_REDIRECT_URI")); v != "" {
		return v
	}
	return "https://mashad.shop/mscale/api/auth/google/callback"
}

var (
	desktopSessions = make(map[string]string)
	desktopMu       sync.Mutex
)

func (h *AuthHandler) LoginGoogleInit(w http.ResponseWriter, r *http.Request) {
	if googleClientID() == "" || googleClientSecret() == "" {
		http.Error(w, "Google OAuth is not configured (set GOOGLE_OAUTH_CLIENT_ID and GOOGLE_OAUTH_CLIENT_SECRET)", http.StatusServiceUnavailable)
		return
	}

	loginType := r.URL.Query().Get("type")
	state := "web"
	if loginType == "desktop" {
		session := strings.TrimSpace(r.URL.Query().Get("session"))
		if session == "" {
			http.Error(w, "missing session", http.StatusBadRequest)
			return
		}
		state = "desktop_" + session
	}

	authURL := fmt.Sprintf(
		"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s&access_type=online&prompt=select_account",
		url.QueryEscape(googleClientID()),
		url.QueryEscape(googleRedirectURI()),
		url.QueryEscape("email profile"),
		url.QueryEscape(state),
	)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) LoginGoogleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" {
		http.Error(w, "Missing code from Google", http.StatusBadRequest)
		return
	}

	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", googleClientID())
	data.Set("client_secret", googleClientSecret())
	data.Set("redirect_uri", googleRedirectURI())
	data.Set("grant_type", "authorization_code")

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", data)
	if err != nil {
		http.Error(w, "Token exchange failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var tokenRes struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenRes); err != nil || tokenRes.AccessToken == "" {
		msg := "Failed to get access token from Google"
		if tokenRes.ErrorDesc != "" {
			msg = tokenRes.ErrorDesc
		} else if tokenRes.Error != "" {
			msg = tokenRes.Error
		}
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)
	userResp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Failed to fetch user info", http.StatusInternalServerError)
		return
	}
	defer userResp.Body.Close()

	var userInfo struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&userInfo); err != nil || userInfo.Email == "" {
		http.Error(w, "No email returned from Google", http.StatusBadRequest)
		return
	}
	userInfo.Email = strings.ToLower(strings.TrimSpace(userInfo.Email))

	var userID string
	err = h.DB.QueryRow("SELECT id FROM users WHERE email = ?", userInfo.Email).Scan(&userID)
	if err == sql.ErrNoRows {
		userID = makeUserID(userInfo.Email)
		username := makeUsernameFromEmail(userInfo.Email)
		hash, _ := utils.HashPassword(fmt.Sprintf("oauth_%d", time.Now().UnixNano()))
		_, err = h.DB.Exec(
			"INSERT INTO users (id, email, username, password_hash, display_name) VALUES (?, ?, ?, ?, ?)",
			userID, userInfo.Email, username, hash, userInfo.Name,
		)
		if err != nil {
			http.Error(w, "Failed to create user in database", http.StatusInternalServerError)
			return
		}
	} else if err != nil {
		http.Error(w, "Database error looking up user", http.StatusInternalServerError)
		return
	}

	sessionID, err := generateSessionID()
	if err != nil {
		http.Error(w, "Failed to generate session", http.StatusInternalServerError)
		return
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = h.DB.Exec(
		"INSERT INTO user_sessions (id, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)",
		sessionID, userID, expiresAt, time.Now(),
	)
	if err != nil {
		http.Error(w, "Failed to store session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		Expires:  expiresAt,
	})

	if strings.HasPrefix(state, "desktop_") {
		desktopSession := strings.TrimPrefix(state, "desktop_")
		desktopMu.Lock()
		desktopSessions[desktopSession] = sessionID
		desktopMu.Unlock()

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><head><title>Success</title><style>body{font-family:sans-serif;text-align:center;margin-top:50px;}</style></head><body><h2>Login Successful!</h2><p>You can close this window and return to the Mscale app.</p><script>setTimeout(function(){window.close();}, 2000);</script></body></html>")
		return
	}

	http.Redirect(w, r, "/mscale/?desktop=1", http.StatusTemporaryRedirect)
}

func (h *AuthHandler) CheckLoginStatus(w http.ResponseWriter, r *http.Request) {
	desktopSession := strings.TrimSpace(r.URL.Query().Get("session"))
	if desktopSession == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "missing session"})
		return
	}

	desktopMu.Lock()
	token, ok := desktopSessions[desktopSession]
	if ok {
		delete(desktopSessions, desktopSession)
	}
	desktopMu.Unlock()

	if !ok {
		writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "success",
		"token":  token,
	})
}
