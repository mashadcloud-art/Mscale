//go:build windows

package main

import (
	"fmt"
	"strings"

	"golang.zx2c4.com/wireguard/tun"
)

func removeStaleMScaleAdapter() {
	script := `
$adapters = Get-NetAdapter -ErrorAction SilentlyContinue | Where-Object {
  $_.Name -like 'MScale*' -or $_.InterfaceDescription -like '*Wintun*'
}
foreach ($a in $adapters) {
  Remove-NetAdapter -Name $a.Name -Confirm:$false -ErrorAction SilentlyContinue
}
`
	hiddenCommand("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script).Run()
}

func createMScaleTUN() (tun.Device, error) {
	if !isProcessElevated() {
		return nil, fmt.Errorf("not running as administrator — close MScale and run mscale-desktop.exe again (accept the UAC prompt), or use Restart as Administrator")
	}
	if err := ensureWintunDLL(); err != nil {
		return nil, fmt.Errorf("wintun driver setup failed: %w", err)
	}

	tunDevice, err := tun.CreateTUN("MScale VPN", 1420)
	if err != nil {
		msg := err.Error()
		lower := strings.ToLower(msg)
		switch {
		case strings.Contains(lower, "access is denied"), strings.Contains(lower, "administrator"):
			return nil, fmt.Errorf("%w — right-click mscale-desktop.exe → Run as administrator", err)
		case strings.Contains(lower, "already exists"), strings.Contains(lower, "in use"):
			removeStaleMScaleAdapter()
			tunDevice, err = tun.CreateTUN("MScale", 1420)
			if err != nil {
				return nil, fmt.Errorf("%w — remove old adapters in Settings → Network → disable/delete MScale, then retry", err)
			}
			return tunDevice, nil
		default:
			return nil, fmt.Errorf("%w — if this persists, reboot or remove MScale/Wintun adapters in Network settings", err)
		}
	}
	return tunDevice, nil
}
