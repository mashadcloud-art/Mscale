package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) LoginWithGoogle() string {
	a.prepareForNewLogin()

	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
	authURL := fmt.Sprintf("%s/api/auth/google/login?type=desktop&session=%s", adminConsoleURL, sessionID)

	runtime.BrowserOpenURL(a.ctx, authURL)

	for i := 0; i < 90; i++ {
		time.Sleep(2 * time.Second)
		req, err := http.NewRequest("GET", apiURL(fmt.Sprintf("/api/auth/google/status?session=%s", sessionID)), nil)
		if err != nil {
			continue
		}
		resp, err := a.httpClient.Do(req)
		if err != nil {
			continue
		}

		var res struct {
			Status string `json:"status"`
			Token  string `json:"token"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&res)
		resp.Body.Close()

		if res.Status == "pending" {
			continue
		}
		if res.Status == "success" && res.Token != "" {
			cookie := &http.Cookie{
				Name:  "mscale_session",
				Value: res.Token,
				Path:  "/",
			}
			urlObj, _ := url.Parse(apiURL("/"))
			a.httpClient.Jar.SetCookies(urlObj, []*http.Cookie{cookie})

			me, err := a.fetchMe()
			if err != nil {
				a.loggedInUser = ""
				a.currentUserEmail = ""
				return "Error: Login succeeded but session was not saved — " + err.Error()
			}

			a.setLoggedInFromMe(me)
			a.persistSessionAfterLogin(me)
			_, _ = a.fetchBridgeAdminURL()
			return "Success: Logged in via Google as " + a.loggedInUser
		}
	}

	return "Error: Google login timed out. Complete sign-in in the browser, then try again."
}
