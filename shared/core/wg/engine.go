package wg

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	WGEndpoint  = "129.151.146.44:51820"
	OverlayCIDR = "100.64.0.0/10"
)

// Engine wraps the WireGuard device.
type Engine struct {
	Device     *device.Device
	PrivateKey wgtypes.Key
	ProtectFD  func(fd int) bool
}

// NewEngine creates a new WireGuard engine wrapper.
func NewEngine(privateKey wgtypes.Key, protectFD func(fd int) bool) *Engine {
	return &Engine{
		PrivateKey: privateKey,
		ProtectFD:  protectFD,
	}
}

// LoadOrCreatePrivateKey loads a key from storage or generates a new one.
func LoadOrCreatePrivateKey(storageDir string) (wgtypes.Key, error) {
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

// Start brings up the WireGuard interface using the provided TUN device.
func (e *Engine) Start(tunDevice tun.Device, serverKeyHex string, isExitMode bool) error {
	serverKey, err := wgtypes.ParseKey(serverKeyHex)
	if err != nil {
		return fmt.Errorf("invalid server key: %w", err)
	}

	logger := device.NewLogger(device.LogLevelError, "[MscaleWG] ")
	var bind conn.Bind = conn.NewDefaultBind()
	if e.ProtectFD != nil {
		bind = NewProtectedBind(e.ProtectFD)
	}

	e.Device = device.NewDevice(tunDevice, bind, logger)

	var wgConfig string
	if isExitMode {
		wgConfig = fmt.Sprintf(
			"private_key=%s\npublic_key=%s\nendpoint=%s\nallowed_ip=0.0.0.0/1\nallowed_ip=128.0.0.0/1\npersistent_keepalive_interval=15\n",
			hex.EncodeToString(e.PrivateKey[:]),
			hex.EncodeToString(serverKey[:]),
			WGEndpoint,
		)
	} else {
		wgConfig = fmt.Sprintf(
			"private_key=%s\npublic_key=%s\nendpoint=%s\nallowed_ip=%s\npersistent_keepalive_interval=25\n",
			hex.EncodeToString(e.PrivateKey[:]),
			hex.EncodeToString(serverKey[:]),
			WGEndpoint,
			OverlayCIDR,
		)
	}

	if err := e.Device.IpcSet(wgConfig); err != nil {
		e.Device.Close()
		e.Device = nil
		return fmt.Errorf("WireGuard config failed: %w", err)
	}
	if err := e.Device.Up(); err != nil {
		e.Device.Close()
		e.Device = nil
		return fmt.Errorf("WireGuard up failed: %w", err)
	}
	return nil
}

// Stop brings down the WireGuard interface.
func (e *Engine) Stop() {
	if e.Device != nil {
		e.Device.Down()
		e.Device.Close()
		e.Device = nil
	}
}
