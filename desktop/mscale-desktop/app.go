package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	serverIP   = "129.151.146.44"
	serverPort = "8081"
)

type App struct {
	ctx          context.Context
	stopHeart    chan struct{}
	heartRunning bool

	assignedIP   string
	deviceID     string
	deviceName   string
	overlayNet   string
	loggedInUser string

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
		return fmt.Sprintf("Error: Could not reach server — %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "Error: " + parseAPIError(resp)
	}

	me, err := a.fetchMe()
	if err != nil {
		a.loggedInUser = ""
		return "Error: Login succeeded but session was not saved — " + err.Error()
	}

	if strings.TrimSpace(me.DisplayName) != "" {
		a.loggedInUser = me.DisplayName + " (" + me.Email + ")"
	} else if strings.TrimSpace(me.Email) != "" {
		a.loggedInUser = me.Email
	} else {
		a.loggedInUser = me.UserID
	}

	return "Success: Logged in as " + a.loggedInUser
}

func (a *App) GetLoggedInUser() string {
	return a.loggedInUser
}

func (a *App) ensureDeviceRecord(mode string, publicKey string) (string, error) {
	payload := deviceRegisterRequest{
		DeviceName: a.deviceName,
		Platform:   "windows",
		DeviceType: "desktop",
		PublicKey:  publicKey,
		AppVersion: "0.1.0",
		OSVersion:  "windows",
		TunnelMode: mode,
		EndpointIP: serverIP,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL("/api/devices/register"), bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		var out deviceRegisterResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return "", fmt.Errorf("invalid register response")
		}
		return out.ID, nil
	}

	return "", fmt.Errorf(parseAPIError(resp))
}

func (a *App) updateDevicePublicKey(deviceID string, publicKey string) error {
	payload := updateKeyRequest{
		DeviceID:  deviceID,
		PublicKey: publicKey,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL("/api/devices/update-key"), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(parseAPIError(resp))
	}

	return nil
}

func (a *App) enrollDevice(deviceID string) (*enrollResponse, error) {
	payload := enrollRequest{
		DeviceID: deviceID,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL("/api/devices/enroll"), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(parseAPIError(resp))
	}

	var out enrollResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("invalid enroll response")
	}
	return &out, nil
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

func (a *App) ConnectTunnel(mode string) string {
	me, err := a.fetchMe()
	if err != nil {
		return "Error: You are not logged in — " + err.Error()
	}
	if strings.TrimSpace(me.DisplayName) != "" {
		a.loggedInUser = me.DisplayName + " (" + me.Email + ")"
	} else if strings.TrimSpace(me.Email) != "" {
		a.loggedInUser = me.Email
	}

	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "Error: Could not generate private key"
	}
	pubKeyBytes := privateKey.PublicKey()
	publicKey := hex.EncodeToString(pubKeyBytes[:])

	deviceID, err := a.ensureDeviceRecord(mode, publicKey)
	if err != nil {
		return "Error: device registration failed — " + err.Error()
	}
	a.deviceID = deviceID

	if err := a.updateDevicePublicKey(deviceID, publicKey); err != nil {
		return "Error: public key update failed — " + err.Error()
	}

	enroll, err := a.enrollDevice(deviceID)
	if err != nil {
		return "Error: device enrollment failed — " + err.Error()
	}

	overlayNet := "100.64.0.0/10"
	if mode == "exit-node" {
		overlayNet = "0.0.0.0/0"
	}
	a.overlayNet = overlayNet

	serverKey, err := wgtypes.ParseKey(strings.TrimSpace(enroll.ServerKey))
	if err != nil {
		return "Error: Invalid server key"
	}

	tunDevice, err := tun.CreateTUN("MScale", 1420)
	if err != nil {
		return "Error: TUN create failed (run as Administrator)"
	}
	a.tunDevice = tunDevice

	adapterName, err := tunDevice.Name()
	if err != nil {
		tunDevice.Close()
		return "Error: get adapter name failed"
	}
	a.adapterName = adapterName

	logger := device.NewLogger(device.LogLevelError, "[WG] ")
	wgEngine := device.NewDevice(tunDevice, conn.NewDefaultBind(), logger)
	a.wgEngine = wgEngine

	wgEndpoint := serverIP + ":51820"
	wgConfig := fmt.Sprintf(
		"private_key=%s\npublic_key=%s\nendpoint=%s\nallowed_ip=%s\npersistent_keepalive_interval=25\n",
		hex.EncodeToString(privateKey[:]),
		hex.EncodeToString(serverKey[:]),
		wgEndpoint,
		overlayNet,
	)

	if err := wgEngine.IpcSet(wgConfig); err != nil {
		wgEngine.Close()
		a.wgEngine = nil
		return "Error: WireGuard config failed — " + err.Error()
	}

	if err := wgEngine.Up(); err != nil {
		wgEngine.Close()
		a.wgEngine = nil
		return "Error: WireGuard up failed — " + err.Error()
	}

	exec.Command("netsh", "interface", "ipv4", "set", "address",
		"name="+adapterName, "static", enroll.OverlayIP, "255.192.0.0").Run()

	exec.Command("netsh", "interface", "ipv4", "add", "route",
		overlayNet, "name="+adapterName, "store=active").Run()

	a.assignedIP = enroll.OverlayIP
	a.startHeartbeatLoop()

	return fmt.Sprintf("Assigned IP: %s\nMode: %s\nLogged in as: %s", enroll.OverlayIP, mode, a.loggedInUser)
}

func (a *App) DisconnectTunnel() string {
	a.stopHeartbeat()

	if a.wgEngine != nil {
		a.wgEngine.Down()
		exec.Command("netsh", "interface", "ipv4", "delete", "route",
			a.overlayNet, "name="+a.adapterName).Run()
		a.wgEngine.Close()
		a.wgEngine = nil
	}

	if a.tunDevice != nil {
		a.tunDevice.Close()
		a.tunDevice = nil
	}

	a.assignedIP = ""
	a.deviceID = ""
	return "Disconnected"
}

func (a *App) GetStatus() string {
	if a.assignedIP == "" {
		if a.loggedInUser != "" {
			return "Logged in as: " + a.loggedInUser
		}
		return "Disconnected"
	}
	return fmt.Sprintf("Connected — IP: %s | %s", a.assignedIP, a.loggedInUser)
}
