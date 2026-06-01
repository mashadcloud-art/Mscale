package mscalecore

import (
	"encoding/hex"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"mscale.core/api"
	"mscale.core/exit"
	"mscale.core/wg"

	"golang.zx2c4.com/wireguard/tun"
)

// VpnController is the main entry point exposed to Android/Kotlin via JNI.
type VpnController struct {
	disconnectMu sync.Mutex
	isRunning   bool
	stopHeart   chan struct{}

	apiClient   *api.Client
	wgEngine    *wg.Engine
	tunDevice   tun.Device

	status       string
	lastError    string
	assignedIP   string
	deviceID     string
	exitNodeID   string
	exitShare    bool
	exitCountry  string
	exitMode     string
	protectFD    func(fd int) bool
	serverKey    string
	sessionToken string
}

func NewVpnController() *VpnController {
	return &VpnController{status: "Disconnected"}
}

func (c *VpnController) GetDeviceID() string     { return c.deviceID }
func (c *VpnController) GetLastError() string      { return c.lastError }
func (c *VpnController) GetAssignedIP() string     { return c.assignedIP }
func (c *VpnController) GetStatus() string         { return c.status }
func (c *VpnController) UsesExitNode() bool        { return strings.TrimSpace(c.exitNodeID) != "" }
func (c *VpnController) IsExitShare() bool         { return c.exitShare }

// SocketProtector is implemented by Android VpnService to protect outbound exit-forward sockets.
type SocketProtector interface {
	Protect(fd int) bool
}

func (c *VpnController) SetSocketProtector(p SocketProtector) {
	if p == nil {
		c.protectFD = nil
		return
	}
	c.protectFD = p.Protect
	installWireGuardProtect(c.protectFD)
	exit.SetSocketProtect(c.protectFD)
}

func (c *VpnController) PrepareMesh(token, deviceName, storageDir, exitNodeID string) string {
	return c.prepareMesh(token, deviceName, storageDir, exitNodeID, false, "", "native")
}

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
		shareCountry = "AE"
	}
	if token == "" {
		c.lastError = "not logged in"
		return ""
	}
	if deviceName == "" {
		deviceName = "Android"
	}

	privateKey, err := wg.LoadOrCreatePrivateKey(storageDir)
	if err != nil {
		c.lastError = "could not load WireGuard key: " + err.Error()
		return ""
	}

	c.wgEngine = wg.NewEngine(privateKey, c.protectFD)

	tunnelMode := "mesh"
	if exitNodeID != "" {
		tunnelMode = "exit-via"
	} else if shareExit {
		tunnelMode = "exit-node"
	}

	c.apiClient = api.NewClient(token, "", exitNodeID != "", c.protectFD)

	pub := privateKey.PublicKey()
	pubHex := hex.EncodeToString(pub[:])

	deviceID, err := c.apiClient.RegisterDevice(deviceName, pubHex, tunnelMode, exitNodeID)
	if err != nil {
		c.lastError = err.Error()
		return ""
	}
	c.deviceID = deviceID

	if err := c.apiClient.UpdateDeviceKey(deviceID, pubHex); err != nil {
		c.lastError = err.Error()
		return ""
	}

	enroll, err := c.apiClient.EnrollDevice(deviceID)
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

	// Defer hub exit routing until WireGuard is up (StartTunnel).
	if exitNodeID != "" {
		c.exitNodeID = exitNodeID
	}

	if shareExit {
		if exitMode != "proxy" {
			exitMode = "proxy"
		}
		if err := c.apiClient.EnableExitNode(deviceID, shareCountry); err != nil {
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
func (t *androidTun) MTU() (int, error)         { return t.mtu, nil }
func (t *androidTun) Name() (string, error)     { return "tun0", nil }
func (t *androidTun) Events() <-chan tun.Event  { return t.events }
func (t *androidTun) Close() error              { return t.file.Close() }
func (t *androidTun) BatchSize() int            { return 1 }

func (c *VpnController) StartTunnel(fd int64) string {
	c.lastError = ""
	if c.isRunning {
		return ""
	}
	if c.assignedIP == "" || c.wgEngine == nil || c.serverKey == "" {
		c.lastError = "call PrepareMesh first"
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
	exit.SetSocketProtect(c.protectFD)

	tunForWG := tun.Device(tunDevice)
	if c.exitShare && c.exitMode == "proxy" {
		pt, err := exit.NewProxyTun(tunDevice, 1280)
		if err == nil {
			tunForWG = pt
		} else {
			c.lastError = "failed to init proxy tun: " + err.Error()
			return c.lastError
		}
	}
	c.tunDevice = tunForWG

	if err := c.wgEngine.Start(tunForWG, c.serverKey, c.exitNodeID != ""); err != nil {
		c.Disconnect()
		c.lastError = err.Error()
		return c.lastError
	}

	if c.exitNodeID != "" {
		pub := c.wgEngine.PrivateKey.PublicKey()
		pubHex := hex.EncodeToString(pub[:])
		if err := c.apiClient.ActivateExitRoute(c.deviceID, c.exitNodeID); err != nil {
			c.Disconnect()
			c.lastError = "exit route activation failed: " + err.Error()
			return c.lastError
		}
		c.apiClient.EnsureExitRouteByKey(pubHex, c.assignedIP, c.exitNodeID)
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
	client := api.NewClient(token, "", false, nil)
	return client.FetchExitNodes()
}

func (c *VpnController) Disconnect() error {
	c.disconnectMu.Lock()
	defer c.disconnectMu.Unlock()

	if !c.isRunning && c.wgEngine == nil && c.tunDevice == nil {
		return nil
	}

	log.Println("Stopping Mscale VPN...")
	c.stopHeartbeat()

	if c.apiClient != nil && c.deviceID != "" {
		if c.exitNodeID != "" {
			_ = c.apiClient.DeactivateExitRoute(c.deviceID)
		}
		if c.exitShare {
			_ = c.apiClient.DisableExitNode(c.deviceID)
		}
		c.apiClient.PostStatus(c.deviceID, "offline")
	}

	if c.wgEngine != nil {
		c.wgEngine.Stop()
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

	if c.apiClient != nil && c.deviceID != "" {
		c.apiClient.PostStatus(c.deviceID, "active")
	}

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		exitNodeID := c.exitNodeID
		for {
			select {
			case <-ticker.C:
				if c.apiClient != nil && c.deviceID != "" {
					c.apiClient.PostStatus(c.deviceID, "active")
					if exitNodeID != "" {
						_ = c.apiClient.ActivateExitRoute(c.deviceID, exitNodeID)
					}
				}
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
