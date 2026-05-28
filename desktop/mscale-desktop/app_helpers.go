package main

import (
	"os"
	"path/filepath"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const exitInternetWG = "0.0.0.0/1,128.0.0.0/1"

type exitNodeListItem struct {
	ID          string  `json:"id"`
	DeviceID    string  `json:"device_id"`
	DeviceName  string  `json:"device_name"`
	Label       string  `json:"label"`
	CountryCode *string `json:"country_code,omitempty"`
	OverlayIP   *string `json:"overlay_ip,omitempty"`
	Status      string  `json:"status"`
	OwnerName   string  `json:"owner_name,omitempty"`
}

func getSystemDNS() string {
	return "8.8.8.8"
}

func desktopWGKeyPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "MScale", "wg_private.key")
}

func loadOrCreateDesktopPrivateKey() (wgtypes.Key, error) {
	keyPath := desktopWGKeyPath()
	if b, err := os.ReadFile(keyPath); err == nil && len(b) > 0 {
		k, err := wgtypes.ParseKey(string(b))
		if err == nil {
			return k, nil
		}
	}
	k, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return k, err
	}
	_ = os.MkdirAll(filepath.Dir(keyPath), 0o700)
	_ = os.WriteFile(keyPath, []byte(k.String()), 0o600)
	return k, nil
}

func saveDeviceID(deviceName, deviceID string) {
	path := filepath.Join(os.Getenv("LOCALAPPDATA"), "MScale", "device_id.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, []byte(deviceID), 0o600)
}
