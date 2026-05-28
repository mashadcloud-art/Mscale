package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

const (
	serverIP   = "129.151.146.44"
	serverPort = "8081"
	adminConsoleURL = "https://mashad.shop/mscale"
)

type App struct {
	ctx          context.Context
	stopHeart    chan struct{}
	heartRunning bool

	assignedIP   string
	deviceID     string
	deviceName   string
	overlayNet   string
	exitGatewayIP string
	loggedInUser     string
	currentUserEmail string

	wgEngine    *device.Device
	tunDevice   tun.Device
	adapterName string

	httpClient *http.Client
}

type apiError struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type meResponse struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
}

type deviceRegisterRequest struct {
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	DeviceType string `json:"device_type"`
	PublicKey  string `json:"public_key"`
	AppVersion string `json:"app_version,omitempty"`
	OSVersion  string `json:"os_version,omitempty"`
	CurrentDNS string `json:"current_dns,omitempty"`
	ExitNodeID string `json:"exit_node_id,omitempty"`
	TunnelMode string `json:"tunnel_mode,omitempty"`
	EndpointIP string `json:"endpoint_ip,omitempty"`
}

type deviceRegisterResponse struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	DeviceType string `json:"device_type"`
	PublicKey  string `json:"public_key"`
	Status     string `json:"status"`
}

type updateKeyRequest struct {
	DeviceID  string `json:"device_id"`
	PublicKey string `json:"public_key"`
}

type enrollRequest struct {
	DeviceID string `json:"device_id"`
}

type enrollResponse struct {
	DeviceID   string `json:"device_id"`
	OverlayIP  string `json:"overlay_ip"`
	ServerKey  string `json:"server_key"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	DeviceName string `json:"device_name"`
}

func NewApp() *App {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 15 * time.Second,
		Jar:     jar,
	}
	return &App{
		stopHeart:  make(chan struct{}),
		httpClient: client,
		deviceName: getHostname(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.tryRestoreSession()
}

func (a *App) shutdown(ctx context.Context) {
	a.stopHeartbeat()

	if a.wgEngine != nil {
		a.wgEngine.Close()
	}
	if a.tunDevice != nil {
		a.tunDevice.Close()
	}
}

func getHostname() string {
	name, err := os.Hostname()
	if err != nil || strings.TrimSpace(name) == "" {
		return "mscale-device"
	}
	return name
}

func apiURL(path string) string {
	return fmt.Sprintf("http://%s:%s%s", serverIP, serverPort, path)
}

func parseAPIError(resp *http.Response) string {
	body, _ := io.ReadAll(resp.Body)

	var e apiError
	if err := json.Unmarshal(body, &e); err == nil && e.Error != "" {
		if e.Details != "" {
			return e.Error + ": " + e.Details
		}
		return e.Error
	}

	if len(body) > 0 {
		return strings.TrimSpace(string(body))
	}

	return fmt.Sprintf("server returned status %d", resp.StatusCode)
}

func (a *App) fetchMe() (*meResponse, error) {
	req, err := http.NewRequest("GET", apiURL("/api/me"), nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(parseAPIError(resp))
	}

	var me meResponse
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return nil, fmt.Errorf("invalid /api/me response")
	}

	return &me, nil
}

func (a *App) Login(email string, password string) string {
	payload := map[string]string{
		"email":    strings.TrimSpace(email),
		"password": password,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL("/api/auth/login"), bytes.NewBuffer(body))
	if err != nil {
		return "Error: Failed to build request"
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("Error: Could not reach server Ã¢â‚¬â€ %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "Error: " + parseAPIError(resp)
	}

	me, err := a.fetchMe()
	if err != nil {
		a.loggedInUser = ""
		a.currentUserEmail = ""
		return "Error: Login succeeded but session was not saved — " + err.Error()
	}

	a.setLoggedInFromMe(me)
	a.persistSessionAfterLogin(me)

	return "Success: Logged in as " + a.loggedInUser
}

func (a *App) GetLoggedInUser() string {
	return a.loggedInUser
}







func (a *App) sendHeartbeat() {
	if a.deviceID == "" {
		return
	}

	payload := map[string]string{
		"status":  "active",
		"peer_id": a.deviceID,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL("/status/update"), bytes.NewBuffer(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func (a *App) startHeartbeatLoop() {
	if a.heartRunning {
		return
	}
	a.heartRunning = true

	go func() {
		a.sendHeartbeat()

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				a.sendHeartbeat()
			case <-a.stopHeart:
				a.heartRunning = false
				return
			}
		}
	}()
}

func (a *App) stopHeartbeat() {
	if a.heartRunning {
		close(a.stopHeart)
		a.stopHeart = make(chan struct{})
		a.heartRunning = false
	}
}






