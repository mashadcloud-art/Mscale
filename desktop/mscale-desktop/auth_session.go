package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type savedAccount struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	LastUsed    int64  `json:"last_used"`
}

type accountsStore struct {
	Active   string         `json:"active"`
	Accounts []savedAccount `json:"accounts"`
}

func mscaleDataDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "MScale")
}

func sessionPath() string {
	return filepath.Join(mscaleDataDir(), "session.json")
}

func accountsPath() string {
	return filepath.Join(mscaleDataDir(), "accounts.json")
}

func sessionPathForEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	h := sha256.Sum256([]byte(email))
	name := hex.EncodeToString(h[:16]) + ".json"
	return filepath.Join(mscaleDataDir(), "sessions", name)
}

func displayNameFromMe(me *meResponse) string {
	if me == nil {
		return "Account"
	}
	if n := strings.TrimSpace(me.DisplayName); n != "" {
		return n
	}
	return friendlyNameFromEmail(me.Email)
}

func friendlyNameFromEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return "Account"
	}
	local := email
	if at := strings.Index(email, "@"); at > 0 {
		local = email[:at]
	}
	local = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(local)
	return titleWords(local)
}

func titleWords(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Account"
	}
	parts := strings.Fields(s)
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		r := []rune(strings.ToLower(p))
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}

func sanitizeAccountDisplay(acct savedAccount) string {
	dn := strings.TrimSpace(acct.DisplayName)
	if dn != "" {
		if idx := strings.Index(dn, " ("); idx > 0 {
			dn = strings.TrimSpace(dn[:idx])
		}
		if !strings.EqualFold(dn, acct.Email) && !strings.Contains(dn, "@") {
			return dn
		}
	}
	return friendlyNameFromEmail(acct.Email)
}

func (a *App) setLoggedInFromMe(me *meResponse) {
	a.currentUserEmail = strings.ToLower(strings.TrimSpace(me.Email))
	if strings.TrimSpace(me.DisplayName) != "" {
		a.loggedInUser = me.DisplayName + " (" + me.Email + ")"
	} else if strings.TrimSpace(me.Email) != "" {
		a.loggedInUser = me.Email
	} else {
		a.loggedInUser = me.UserID
	}
}

func (a *App) persistSessionForUser(email string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return
	}

	urlObj, _ := url.Parse(apiURL("/"))
	cookies := a.httpClient.Jar.Cookies(urlObj)
	for _, c := range cookies {
		if c.Name != "mscale_session" || c.Value == "" {
			continue
		}
		path := sessionPathForEmail(email)
		_ = os.MkdirAll(filepath.Dir(path), 0o700)
		_ = os.WriteFile(path, []byte(c.Value), 0o600)
		// Legacy single-session file for older builds.
		_ = os.MkdirAll(filepath.Dir(sessionPath()), 0o700)
		_ = os.WriteFile(sessionPath(), []byte(c.Value), 0o600)
		break
	}
}

func (a *App) persistSessionAfterLogin(me *meResponse) {
	if me == nil || strings.TrimSpace(me.Email) == "" {
		return
	}
	a.persistSessionForUser(me.Email)
	a.upsertSavedAccount(me.Email, displayNameFromMe(me))
}

func (a *App) saveSessionCookie() {
	if a.currentUserEmail != "" {
		a.persistSessionForUser(a.currentUserEmail)
		return
	}
	urlObj, _ := url.Parse(apiURL("/"))
	cookies := a.httpClient.Jar.Cookies(urlObj)
	for _, c := range cookies {
		if c.Name == "mscale_session" && c.Value != "" {
			_ = os.MkdirAll(filepath.Dir(sessionPath()), 0o700)
			_ = os.WriteFile(sessionPath(), []byte(c.Value), 0o600)
			break
		}
	}
}

func (a *App) loadSessionCookie() {
	a.loadSessionForEmail("")
}

func (a *App) loadSessionForEmail(email string) error {
	path := sessionPath()
	if email != "" {
		path = sessionPathForEmail(email)
	}
	val, err := os.ReadFile(path)
	if err != nil || len(val) == 0 {
		if email == "" {
			return err
		}
		return os.ErrNotExist
	}
	cookie := &http.Cookie{
		Name:  "mscale_session",
		Value: string(val),
		Path:  "/",
	}
	urlObj, _ := url.Parse(apiURL("/"))
	a.httpClient.Jar.SetCookies(urlObj, []*http.Cookie{cookie})
	if email != "" {
		a.currentUserEmail = strings.ToLower(strings.TrimSpace(email))
	}
	return nil
}

func (a *App) readAccountsStore() accountsStore {
	data, err := os.ReadFile(accountsPath())
	if err != nil {
		return accountsStore{Accounts: []savedAccount{}}
	}
	var store accountsStore
	if err := json.Unmarshal(data, &store); err != nil {
		return accountsStore{Accounts: []savedAccount{}}
	}
	if store.Accounts == nil {
		store.Accounts = []savedAccount{}
	}
	return store
}

func (a *App) writeAccountsStore(store accountsStore) {
	_ = os.MkdirAll(filepath.Dir(accountsPath()), 0o700)
	body, _ := json.Marshal(store)
	_ = os.WriteFile(accountsPath(), body, 0o600)
}

func (a *App) upsertSavedAccount(email, display string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return
	}
	store := a.readAccountsStore()
	now := time.Now().Unix()
	found := false
	for i := range store.Accounts {
		if strings.EqualFold(store.Accounts[i].Email, email) {
			store.Accounts[i].LastUsed = now
			if strings.TrimSpace(display) != "" {
				store.Accounts[i].DisplayName = display
			}
			found = true
			break
		}
	}
	if !found {
		store.Accounts = append(store.Accounts, savedAccount{
			Email:       email,
			DisplayName: display,
			LastUsed:    now,
		})
	}
	store.Active = email
	a.writeAccountsStore(store)
}

func (a *App) clearActiveAccount() {
	store := a.readAccountsStore()
	store.Active = ""
	a.writeAccountsStore(store)
}

func (a *App) tryRestoreSession() {
	store := a.readAccountsStore()
	email := strings.TrimSpace(store.Active)
	if email == "" {
		_ = a.loadSessionForEmail("")
	} else {
		if err := a.loadSessionForEmail(email); err != nil {
			_ = a.loadSessionForEmail("")
		}
	}
	me, err := a.fetchMe()
	if err != nil {
		a.loggedInUser = ""
		a.currentUserEmail = ""
		return
	}
	a.setLoggedInFromMe(me)
}

func (a *App) clearSessionCookies() {
	urlObj, _ := url.Parse(apiURL("/"))
	a.httpClient.Jar.SetCookies(urlObj, nil)
}

func (a *App) Logout() string {
	a.stopHeartbeat()
	if a.assignedIP != "" || a.wgEngine != nil {
		a.teardownTunnel()
	}

	email := a.currentUserEmail
	a.clearSessionCookies()
	a.loggedInUser = ""
	a.currentUserEmail = ""
	a.clearActiveAccount()

	if email != "" {
		_ = os.Remove(sessionPathForEmail(email))
	} else {
		_ = os.Remove(sessionPath())
	}

	go func() {
		req, err := http.NewRequest("POST", apiURL("/api/auth/logout"), nil)
		if err != nil {
			return
		}
		client := &http.Client{Timeout: 4 * time.Second}
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
		}
	}()

	return "Success"
}

// ListSavedAccountsJSON returns saved accounts for the login screen picker.
func (a *App) ListSavedAccountsJSON() string {
	store := a.readAccountsStore()
	if len(store.Accounts) == 0 {
		return "[]"
	}
	// Most recently used first.
	accounts := make([]savedAccount, len(store.Accounts))
	copy(accounts, store.Accounts)
	for i := range accounts {
		accounts[i].DisplayName = sanitizeAccountDisplay(accounts[i])
	}
	for i := 0; i < len(accounts)-1; i++ {
		for j := i + 1; j < len(accounts); j++ {
			if accounts[j].LastUsed > accounts[i].LastUsed {
				accounts[i], accounts[j] = accounts[j], accounts[i]
			}
		}
	}
	body, err := json.Marshal(accounts)
	if err != nil {
		return "[]"
	}
	return string(body)
}

// SwitchAccount loads a saved session for another account.
func (a *App) SwitchAccount(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "Error: email is required"
	}

	a.stopHeartbeat()
	a.teardownTunnel()
	a.clearSessionCookies()

	if err := a.loadSessionForEmail(email); err != nil {
		return "Error: no saved session — sign in with your password"
	}

	me, err := a.fetchMe()
	if err != nil {
		a.clearSessionCookies()
		_ = os.Remove(sessionPathForEmail(email))
		return "Error: session expired — sign in again"
	}

	a.setLoggedInFromMe(me)
	a.upsertSavedAccount(email, displayNameFromMe(me))
	return "Success: Switched to " + a.loggedInUser
}

// RemoveSavedAccount removes a saved account and its session file.
func (a *App) RemoveSavedAccount(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "Error: email is required"
	}
	store := a.readAccountsStore()
	next := make([]savedAccount, 0, len(store.Accounts))
	for _, acct := range store.Accounts {
		if !strings.EqualFold(acct.Email, email) {
			next = append(next, acct)
		}
	}
	store.Accounts = next
	if strings.EqualFold(store.Active, email) {
		store.Active = ""
	}
	a.writeAccountsStore(store)
	_ = os.Remove(sessionPathForEmail(email))
	return "Success"
}
