package mscalecore

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	apiBase       = "https://mashad.shop/mscale"
	wgEndpoint    = "129.151.146.44:51820"
	overlayCIDR   = "100.64.0.0/10"
	appVersion    = "1.0.11"
	sessionCookie = "mscale_session"
)

// VpnController is the main entry point exposed to Android/Kotlin.
type VpnController struct {
	disconnectMu sync.Mutex
	isRunning   bool
	stopHeart   chan struct{}
	wgEngine    *device.Device
	tunDevice   tun.Device
	status      string
	lastError   string
	assignedIP  string
	deviceID    string
	exitNodeID  string
	exitShare   bool
	exitCountry string
	exitMode    string
	protectFD   func(fd int) bool
	serverKey   string
	sessionToken string
	privateKey  wgtypes.Key
}

type enrollPayload struct {
	OverlayIP string `json:"overlay_ip"`
	ServerKey string `json:"server_key"`
}

func NewVpnController() *VpnController {
	return &VpnController{status: "Disconnected"}
}

func (c *VpnController) GetDeviceID() string      { return c.deviceID }
func (c *VpnController) GetLastError() string       { return c.lastError }
func (c *VpnController) GetAssignedIP() string  { return c.assignedIP }
func (c *VpnController) GetStatus() string      { return c.status }
func (c *VpnController) UsesExitNode() bool     { return strings.TrimSpace(c.exitNodeID) != "" }
func (c *VpnController) IsExitShare() bool      { return c.exitShare }

// SocketProtector is implemented by Android VpnService to protect outbound exit-forward sockets.
type SocketProtector interface {
	Protect(fd int) bool
}

// SetSocketProtector wires VpnService.protect() for WireGuard + exit forwarding.
func (c *VpnController) SetSocketProtector(p SocketProtector) {
	if p == nil {
		c.protectFD = nil
		return
	}
	c.protectFD = p.Protect
	installWireGuardProtect(c.protectFD)
}

// PrepareMesh registers, enrolls, optionally activates exit routing or registers as exit provider.
func (c *VpnController) PrepareMesh(token, deviceName, storageDir, exitNodeID string) string {
	return c.prepareMesh(token, deviceName, storageDir, exitNodeID, false, "", "native")
}

// PrepareMeshAsExit connects to mesh and registers this device as an exit node (phone/tablet).
func (c *VpnController) PrepareMeshAsExit(token, deviceName, storageDir, countryCode, exitMode string) string {
	return c.prepareMesh(token, deviceName, storageDir, "", true, countryCode, exitMode)
}

func (c *VpnController) prepareMesh(token, deviceName, storageDir, exitNodeID string, shareExit bool, shareCountry string, exitMode string) string {
	c.lastError = ""
	c.assignedIP = ""
	c.deviceID = ""
	c.exitNodeID = ""
	c.exitShare = false
	c.exitCountry = ""
	c.exitMode = ""
	c.sessionToken = strings.TrimSpace(token)
	c.stopHeartbeat()

	token = c.sessionToken
	deviceName = strings.TrimSpace(deviceName)
	exitNodeID = strings.TrimSpace(exitNodeID)
	shareCountry = strings.ToUpper(strings.TrimSpace(shareCountry))
	if shareExit && shareCountry == "" {
		shareCountry = "IN"
	}
	if token == "" {
		c.lastError = "not logged in"
		return ""
	}
	if deviceName == "" {
		deviceName = "Android"
	}

	privateKey, err := loadOrCreatePrivateKey(storageDir)
	if err != nil {
		c.lastError = "could not load WireGuard key: " + err.Error()
		return ""
	}
	c.privateKey = privateKey

	pub := privateKey.PublicKey()
	publicKeyHex := hex.EncodeToString(pub[:])

	tunnelMode := "mesh"
	if exitNodeID != "" {
		tunnelMode = "exit-via"
	} else if shareExit {
		tunnelMode = "exit-node"
	}

	deviceID, err := c.registerDevice(token, deviceName, publicKeyHex, tunnelMode, exitNodeID)
	if err != nil {
		c.lastError = err.Error()
		return ""
	}
	c.deviceID = deviceID

	if err := c.updateDeviceKey(token, deviceID, publicKeyHex); err != nil {
		c.lastError = err.Error()
		return ""
	}

	enroll, err := c.enrollDevice(token, deviceID)
	if err != nil {
		c.lastError = err.Error()
		return ""
	}

	c.assignedIP = strings.TrimSpace(enroll.OverlayIP)
	c.serverKey = strings.TrimSpace(enroll.ServerKey)
	if c.assignedIP == "" || c.serverKey == "" {
		c.lastError = "server did not return mesh configuration"
		return ""
	}

	if exitNodeID != "" {
		if err := c.activateExitRoute(token, deviceID, exitNodeID); err != nil {
			c.lastError = err.Error()
			return ""
		}
		c.exitNodeID = exitNodeID
		c.ensureExitRouteByKey(token, publicKeyHex, c.assignedIP)
	}

	if shareExit {
		if err := c.enableExitNode(token, deviceID, shareCountry); err != nil {
			c.lastError = err.Error()
			return ""
		}
		c.exitShare = true
		c.exitCountry = shareCountry
		c.exitMode = exitMode
	}

	c.status = "Ready"
	return c.assignedIP
}

type androidTun struct {
	file   *os.File
	mtu    int
	events chan tun.Event
}

func (t *androidTun) File() *os.File { return t.file }
func (t *androidTun) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	if len(bufs) == 0 {
		return 0, nil
	}
	for {
		n, err := t.file.Read(bufs[0][offset:])
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "EAGAIN") || strings.Contains(errStr, "EINTR") {
				continue
			}
			if strings.Contains(errStr, "EBADF") || strings.Contains(errStr, "bad file descriptor") {
				return 0, io.EOF
			}
			return 0, err
		}
		sizes[0] = n
		return 1, nil
	}
}
func (t *androidTun) Write(bufs [][]byte, offset int) (int, error) {
	for i, buf := range bufs {
		if _, err := t.file.Write(buf[offset:]); err != nil {
			return i, err
		}
	}
	return len(bufs), nil
}
func (t *androidTun) MTU() (int, error)              { return t.mtu, nil }
func (t *androidTun) Name() (string, error)          { return "tun0", nil }
func (t *androidTun) Events() <-chan tun.Event       { return t.events }
func (t *androidTun) Close() error                   { return t.file.Close() }
func (t *androidTun) BatchSize() int                 { return 1 }

// StartTunnel applies WireGuard on the Android TUN file descriptor.
func (c *VpnController) StartTunnel(fd int64) string {
	c.lastError = ""
	if c.isRunning {
		return ""
	}
	if c.assignedIP == "" || c.privateKey.String() == "" || c.serverKey == "" {
		c.lastError = "call PrepareMesh first"
		return c.lastError
	}

	serverKey, err := wgtypes.ParseKey(c.serverKey)
	if err != nil {
		c.lastError = "invalid server key"
		return c.lastError
	}

	tunFile := os.NewFile(uintptr(fd), "tun")
	tunDevice := &androidTun{
		file:   tunFile,
		mtu:    1280,
		events: make(chan tun.Event, 1),
	}
	tunDevice.events <- tun.EventUp

	installWireGuardProtect(c.protectFD)

	tunForWG := tun.Device(tunDevice)
	if c.exitShare && c.exitMode == "proxy" {
		pt, err := NewProxyTun(tunDevice, 1280)
		if err == nil {
			tunForWG = pt
		} else {
			c.lastError = "failed to init proxy tun: " + err.Error()
			return c.lastError
		}
	}
	c.tunDevice = tunForWG

	logger := device.NewLogger(device.LogLevelError, "[MscaleWG] ")
	var bind conn.Bind = conn.NewDefaultBind()
	if c.protectFD != nil {
		bind = newProtectedBind(c.protectFD)
	}
	wgEngine := device.NewDevice(tunForWG, bind, logger)
	c.wgEngine = wgEngine

	var wgConfig string
	if c.exitNodeID != "" {
		wgConfig = fmt.Sprintf(
			"private_key=%s\npublic_key=%s\nendpoint=%s\nallowed_ip=0.0.0.0/1\nallowed_ip=128.0.0.0/1\npersistent_keepalive_interval=15\n",
			hex.EncodeToString(c.privateKey[:]),
			hex.EncodeToString(serverKey[:]),
			wgEndpoint,
		)
	} else {
		wgConfig = fmt.Sprintf(
			"private_key=%s\npublic_key=%s\nendpoint=%s\nallowed_ip=%s\npersistent_keepalive_interval=25\n",
			hex.EncodeToString(c.privateKey[:]),
			hex.EncodeToString(serverKey[:]),
			wgEndpoint,
			overlayCIDR,
		)
	}

	if err := wgEngine.IpcSet(wgConfig); err != nil {
		c.Disconnect()
		c.lastError = "WireGuard config failed: " + err.Error()
		return c.lastError
	}
	if err := wgEngine.Up(); err != nil {
		c.Disconnect()
		c.lastError = "WireGuard up failed: " + err.Error()
		return c.lastError
	}

	if c.exitNodeID != "" {
		pub := c.privateKey.PublicKey()
		c.ensureExitRouteByKey(c.sessionToken, hex.EncodeToString(pub[:]), c.assignedIP)
	}

	c.isRunning = true
	if c.exitShare {
		c.status = "Sharing exit (" + c.exitCountry + ") — " + c.assignedIP
	} else if c.exitNodeID != "" {
		c.status = "Connected via exit — " + c.assignedIP
	} else {
		c.status = "Connected — " + c.assignedIP
	}
	c.startHeartbeat()
	return ""
}

func (c *VpnController) FetchExitNodes(token string) string {
	resp, err := c.apiCall(token, http.MethodGet, "/api/exit-nodes", nil)
	if err != nil {
		return "[]"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "[]"
	}
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

func (c *VpnController) Disconnect() error {
	c.disconnectMu.Lock()
	defer c.disconnectMu.Unlock()

	if !c.isRunning && c.wgEngine == nil && c.tunDevice == nil {
		return nil
	}

	log.Println("Stopping Mscale VPN...")
	c.stopHeartbeat()

	if c.exitNodeID != "" && c.deviceID != "" && c.sessionToken != "" {
		_ = c.deactivateExitRoute(c.sessionToken, c.deviceID)
	}
	if c.exitShare && c.deviceID != "" && c.sessionToken != "" {
		_ = c.disableExitNode(c.sessionToken, c.deviceID)
	}
	if c.deviceID != "" && c.sessionToken != "" {
		c.postStatus(c.sessionToken, c.deviceID, "offline")
	}

	// Stop WireGuard before closing TUN (closing TUN first crashes native readers).
	if c.wgEngine != nil {
		c.wgEngine.Down()
		c.wgEngine.Close()
		c.wgEngine = nil
	}
	if c.tunDevice != nil {
		_ = c.tunDevice.Close()
		c.tunDevice = nil
	}

	c.isRunning = false
	c.exitNodeID = ""
	c.exitShare = false
	c.exitCountry = ""
	c.status = "Disconnected"
	return nil
}

func (c *VpnController) startHeartbeat() {
	c.stopHeartbeat()
	c.stopHeart = make(chan struct{})
	token := c.sessionToken
	deviceID := c.deviceID
	c.postStatus(token, deviceID, "active")

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.postStatus(token, deviceID, "active")
			case <-c.stopHeart:
				return
			}
		}
	}()
}

func (c *VpnController) stopHeartbeat() {
	if c.stopHeart == nil {
		return
	}
	ch := c.stopHeart
	c.stopHeart = nil
	close(ch)
}

func (c *VpnController) postStatus(token, deviceID, status string) {
	if token == "" || deviceID == "" {
		return
	}
	payload, _ := json.Marshal(map[string]string{"status": status, "peer_id": deviceID})
	resp, err := c.apiCall(token, http.MethodPost, "/status/update", payload)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func loadOrCreatePrivateKey(storageDir string) (wgtypes.Key, error) {
	storageDir = strings.TrimSpace(storageDir)
	if storageDir == "" {
		storageDir = os.TempDir()
	}
	keyPath := filepath.Join(storageDir, "wg_private.key")
	if b, err := os.ReadFile(keyPath); err == nil && len(b) > 0 {
		if k, err := wgtypes.ParseKey(strings.TrimSpace(string(b))); err == nil {
			return k, nil
		}
	}
	k, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return k, err
	}
	_ = os.MkdirAll(storageDir, 0o700)
	_ = os.WriteFile(keyPath, []byte(k.String()), 0o600)
	return k, nil
}

func (c *VpnController) apiCall(token, method, path string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, apiBase+path, reader)
	if err != nil {
		return nil, err
	}
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := http.DefaultClient
	if c.exitNodeID != "" && c.protectFD != nil {
		client = protectedHTTPClient(c.protectFD)
	}
	return client.Do(req)
}

func protectedHTTPClient(protect func(fd int) bool) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := net.Dialer{}
				conn, err := d.DialContext(ctx, network, addr)
				if err == nil {
					protectConn(conn, protect)
				}
				return conn, err
			},
		},
	}
}

func protectConn(conn net.Conn, protect func(fd int) bool) {
	if protect == nil {
		return
	}
	rc, ok := conn.(syscall.Conn)
	if !ok {
		return
	}
	raw, err := rc.SyscallConn()
	if err != nil {
		return
	}
	_ = raw.Control(func(fd uintptr) {
		protect(int(fd))
	})
}

func parseAPIError(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	var out struct {
		Error   string `json:"error"`
		Details string `json:"details"`
	}
	if json.Unmarshal(b, &out) == nil && out.Error != "" {
		if out.Details != "" {
			return out.Error + ": " + out.Details
		}
		return out.Error
	}
	return fmt.Sprintf("HTTP %d", resp.StatusCode)
}

func (c *VpnController) registerDevice(token, deviceName, publicKeyHex, tunnelMode, exitNodeID string) (string, error) {
	payload := map[string]string{
		"device_name": deviceName,
		"platform":    "android",
		"device_type": "mobile",
		"public_key":  publicKeyHex,
		"app_version": appVersion,
		"os_version":  "android",
		"tunnel_mode": tunnelMode,
		"endpoint_ip": "129.151.146.44",
	}
	if exitNodeID != "" {
		payload["exit_node_id"] = exitNodeID
	}
	body, _ := json.Marshal(payload)
	resp, err := c.apiCall(token, http.MethodPost, "/api/devices/register", body)
	if err != nil {
		return "", fmt.Errorf("register failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("register: %s", parseAPIError(resp))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.ID == "" {
		return "", fmt.Errorf("invalid register response")
	}
	return out.ID, nil
}

func (c *VpnController) updateDeviceKey(token, deviceID, publicKeyHex string) error {
	payload := map[string]string{
		"device_id":  deviceID,
		"public_key": publicKeyHex,
	}
	body, _ := json.Marshal(payload)
	resp, err := c.apiCall(token, http.MethodPost, "/api/devices/update-key", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update-key: %s", parseAPIError(resp))
	}
	return nil
}

func (c *VpnController) enrollDevice(token, deviceID string) (*enrollPayload, error) {
	payload := map[string]string{"device_id": deviceID}
	body, _ := json.Marshal(payload)
	resp, err := c.apiCall(token, http.MethodPost, "/api/devices/enroll", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("enroll: %s", parseAPIError(resp))
	}
	var out enrollPayload
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("invalid enroll response")
	}
	return &out, nil
}

func (c *VpnController) activateExitRoute(token, deviceID, exitNodeID string) error {
	payload := map[string]string{
		"device_id":    deviceID,
		"exit_node_id": exitNodeID,
	}
	body, _ := json.Marshal(payload)
	resp, err := c.apiCall(token, http.MethodPost, "/api/exit-route/activate", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("exit route: %s", parseAPIError(resp))
	}
	return nil
}

func (c *VpnController) ensureExitRouteByKey(token, publicKeyHex, overlayIP string) {
	if publicKeyHex == "" || overlayIP == "" {
		return
	}
	payload := map[string]string{
		"public_key": publicKeyHex,
		"overlay_ip": overlayIP,
	}
	body, _ := json.Marshal(payload)
	resp, err := c.apiCall(token, http.MethodPost, "/api/exit-route/ensure-by-key", body)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func (c *VpnController) deactivateExitRoute(token, deviceID string) error {
	payload := map[string]string{"device_id": deviceID}
	body, _ := json.Marshal(payload)
	resp, err := c.apiCall(token, http.MethodPost, "/api/exit-route/deactivate", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *VpnController) enableExitNode(token, deviceID, countryCode string) error {
	label := countryCode + " mobile exit"
	payload := map[string]interface{}{
		"device_id":    deviceID,
		"label":        label,
		"country_code": countryCode,
		"is_private":   false,
	}
	body, _ := json.Marshal(payload)
	resp, err := c.apiCall(token, http.MethodPost, "/api/devices/exit-node/enable", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("enable exit: %s", parseAPIError(resp))
	}
	return nil
}

func (c *VpnController) disableExitNode(token, deviceID string) error {
	payload := map[string]string{"device_id": deviceID}
	body, _ := json.Marshal(payload)
	resp, err := c.apiCall(token, http.MethodPost, "/api/devices/exit-node/disable", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
