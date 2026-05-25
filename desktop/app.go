package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
	ctx         context.Context
	stopHeart   chan struct{}
	assignedIP  string
	wgEngine    *device.Device
	tunDevice   tun.Device
	adapterName string
	overlayNet  string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		stopHeart: make(chan struct{}),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.startHeartbeatLoop()
}

// shutdown is called when the app closes — stops the heartbeat goroutine cleanly
func (a *App) shutdown(ctx context.Context) {
	close(a.stopHeart)
}

// getHostname returns the machine hostname for peer identification
func getHostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown-peer"
	}
	return name
}

// sendHeartbeat performs the network request to the server
func (a *App) sendHeartbeat() {
	url := fmt.Sprintf("http://%s:%s/status/update", serverIP, serverPort)

	payload := map[string]string{
		"status":  "active",
		"peer_id": getHostname(),
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		log.Printf("HEARTBEAT: Failed to build request: %v", err)
		return
	}

	token := os.Getenv("MSCALE_TOKEN")
	if token == "" {
		token = "my-secret-token-123"
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("HEARTBEAT: Request failed: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("HEARTBEAT: Sent — server responded %d", resp.StatusCode)
}

// startHeartbeatLoop runs in the background every 60s
// Sends one heartbeat immediately on start, then every 60s
func (a *App) startHeartbeatLoop() {
	a.sendHeartbeat() // send immediately on startup

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.sendHeartbeat()
		case <-a.stopHeart:
			log.Println("HEARTBEAT: Loop stopped")
			return
		}
	}
}

// ConnectTunnel is called from Javascript
func (a *App) ConnectTunnel(token string, mode string) string {
	if token == "" {
		return "Error: Token cannot be empty"
	}

	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "Error: Could not generate private key"
	}

	url := fmt.Sprintf("http://%s:%s/register", serverIP, serverPort)
	pubKey := privateKey.PublicKey()
	payload := map[string]string{
		"public_key": hex.EncodeToString(pubKey[:]),
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "Error: Failed to build request"
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Error: Could not reach server — %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Error: Server returned %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)

	var peer struct {
		IP        string `json:"ip"`
		ServerKey string `json:"server_key"`
	}
	if err := json.Unmarshal(respBody, &peer); err != nil {
		return "Error: Invalid response from server"
	}

	// Determine routing mode
	overlayNet := "100.64.0.0/10" // Default to mesh only
	if mode == "exit-node" {
		overlayNet = "0.0.0.0/0" // Route all traffic
	}
	a.overlayNet = overlayNet

	serverKey, err := wgtypes.ParseKey(strings.TrimSpace(peer.ServerKey))
	if err != nil {
		return "Error: Invalid server key"
	}

	// Create TUN adapter
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

	// Configure WireGuard engine
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
		return "Error: WireGuard config failed"
	}

	if err := wgEngine.Up(); err != nil {
		wgEngine.Close()
		return "Error: WireGuard up failed"
	}

	// Assign IP to adapter
	exec.Command("netsh", "interface", "ipv4", "set", "address",
		"name="+adapterName, "static", peer.IP, "255.192.0.0").Run()

	// Add route
	exec.Command("netsh", "interface", "ipv4", "add", "route",
		overlayNet, "name="+adapterName, "store=active").Run()

	a.assignedIP = peer.IP
	return fmt.Sprintf("Assigned IP: %s\nMode: %s\nServer Key: %s", peer.IP, mode, peer.ServerKey)
}

// DisconnectTunnel is called from Javascript
func (a *App) DisconnectTunnel() string {
	if a.wgEngine != nil {
		a.wgEngine.Down()
		exec.Command("netsh", "interface", "ipv4", "delete", "route",
			a.overlayNet, "name="+a.adapterName).Run()
		a.tunDevice.Close()
		a.wgEngine = nil
		a.tunDevice = nil
	}
	a.assignedIP = ""
	return "Disconnected"
}

// GetStatus returns current connection status to Javascript
func (a *App) GetStatus() string {
	if a.assignedIP == "" {
		return "Disconnected"
	}
	return fmt.Sprintf("Connected — IP: %s", a.assignedIP)
}
