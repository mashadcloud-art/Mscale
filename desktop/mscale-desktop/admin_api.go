package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type bridgeTokenResponse struct {
	BridgeURL string `json:"bridge_url"`
}

// PrepareAdminSession pre-warms a one-time bridge token after desktop login.
func (a *App) PrepareAdminSession() string {
	if !a.IsLoggedIn() {
		return "Error: not logged in"
	}
	_, err := a.fetchBridgeAdminURL()
	if err != nil {
		return "Warn: " + err.Error()
	}
	return "Success"
}

// GetAdminConsoleURL returns a one-time bridge URL that auto-signs into the web admin.
func (a *App) GetAdminConsoleURL() string {
	url, err := a.fetchBridgeAdminURL()
	if err != nil {
		return adminConsoleURL
	}
	return url
}

func (a *App) fetchBridgeAdminURL() (string, error) {
	req, err := http.NewRequest("POST", apiURL("/api/auth/bridge-token"), nil)
	if err != nil {
		return "", err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(parseAPIError(resp))
	}
	var out bridgeTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.BridgeURL) == "" {
		return adminConsoleURL, nil
	}
	return out.BridgeURL, nil
}

// IsLoggedIn reports whether the desktop app has a valid server session.
func (a *App) IsLoggedIn() bool {
	me, err := a.fetchMe()
	if err != nil {
		return false
	}
	if strings.TrimSpace(me.Email) != "" {
		a.loggedInUser = me.Email
	}
	return true
}

// GetSessionUserJSON returns the current user as JSON for the admin console.
func (a *App) GetSessionUserJSON() string {
	me, err := a.fetchMe()
	if err != nil {
		return ""
	}
	if strings.TrimSpace(me.DisplayName) != "" {
		a.loggedInUser = me.DisplayName + " (" + me.Email + ")"
	} else if strings.TrimSpace(me.Email) != "" {
		a.loggedInUser = me.Email
	}
	body, err := json.Marshal(me)
	if err != nil {
		return ""
	}
	return string(body)
}

// ListDevicesJSON returns the user's devices from the server as JSON.
func (a *App) ListDevicesJSON() string {
	req, err := http.NewRequest("GET", apiURL("/api/devices"), nil)
	if err != nil {
		return "[]"
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "[]"
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "[]"
	}
	return string(body)
}

// RegisterAdminDevice registers a new device on the server (admin console "Add device").
func (a *App) RegisterAdminDevice(deviceName, platform string) string {
	deviceName = strings.TrimSpace(deviceName)
	platform = strings.ToLower(strings.TrimSpace(platform))
	if deviceName == "" || platform == "" {
		return "Error: device name and platform are required"
	}

	priv, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "Error: could not generate WireGuard key"
	}

	deviceType := "desktop"
	switch platform {
	case "linux", "server", "agent":
		deviceType = "agent"
	case "ios", "android":
		deviceType = "mobile"
	}

	payload := deviceRegisterRequest{
		DeviceName: deviceName,
		Platform:   platform,
		DeviceType: deviceType,
		PublicKey:  priv.PublicKey().String(),
		AppVersion: GetAppVersion(),
		OSVersion:  platform,
		TunnelMode: "mesh",
		EndpointIP: serverIP,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL("/api/devices/register"), bytes.NewBuffer(body))
	if err != nil {
		return "Error: failed to build request"
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("Error: could not reach server — %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "Error: " + parseAPIError(resp)
	}
	return "Success: Device registered"
}

// AdminAPIJSON proxies an authenticated API call for the embedded admin console.
func (a *App) AdminAPIJSON(path, method, body string) string {
	path = strings.TrimSpace(path)
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "GET"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	var bodyReader io.Reader
	if strings.TrimSpace(body) != "" {
		bodyReader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, apiURL(path), bodyReader)
	if err != nil {
		out, _ := json.Marshal(map[string]interface{}{"status": 500, "body": map[string]string{"error": err.Error()}})
		return string(out)
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		out, _ := json.Marshal(map[string]interface{}{"status": 503, "body": map[string]string{"error": err.Error()}})
		return string(out)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var parsed interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			parsed = map[string]string{"raw": string(respBody)}
		}
	} else {
		parsed = map[string]string{}
	}
	out, _ := json.Marshal(map[string]interface{}{"status": resp.StatusCode, "body": parsed})
	return string(out)
}

// AdminLogout clears the server session from the desktop app.
func (a *App) AdminLogout() string {
	req, err := http.NewRequest("POST", apiURL("/api/auth/logout"), nil)
	if err != nil {
		return "Error: failed to build logout request"
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.loggedInUser = ""
		return fmt.Sprintf("Error: %v", err)
	}
	resp.Body.Close()
	a.loggedInUser = ""
	return "Success"
}
