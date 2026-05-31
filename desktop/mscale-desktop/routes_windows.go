//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"strings"
)

const meshOverlayNet = "100.64.0.0/10"

// applyMeshRoutes adds only the overlay subnet (safe; used before connectivity check).
func applyMeshRoutes(adapterName string) {
	hiddenCommand("netsh", "interface", "ipv4", "add", "route",
		meshOverlayNet, "name="+adapterName, "store=active").Run()
}

// applyExitTunnelRoutes adds internet-via-VPN routes after mesh is confirmed working.
func applyExitTunnelRoutes(adapterName string) {
	// Add explicit host route for the server to prevent VPN loops and triangular routing of API calls
	hiddenCommand("powershell", "-NoProfile", "-Command",
		"$gw = (Get-NetRoute -DestinationPrefix 0.0.0.0/0 | Sort-Object RouteMetric | Select-Object -First 1).NextHop; "+
			"if ($gw) { route add "+serverIP+" mask 255.255.255.255 $gw }").Run()

	for _, prefix := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		hiddenCommand("netsh", "interface", "ipv4", "add", "route",
			"prefix="+prefix, "interface="+adapterName, "metric=1", "store=active").Run()
	}
	hiddenCommand("powershell", "-NoProfile", "-Command",
		"$vpn='"+adapterName+"'; "+
			"Set-NetIPInterface -InterfaceAlias $vpn -InterfaceMetric 1 -ErrorAction SilentlyContinue; "+
			"Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.Name -ne $vpn } | "+
			"ForEach-Object { Set-NetIPInterface -InterfaceAlias $_.Name -InterfaceMetric 50 -ErrorAction SilentlyContinue }").Run()
}

// applyWindowsTunnelRoutes adds VPN routes. Does NOT delete Wi-Fi/Ethernet defaults (avoids total blackout).
func applyWindowsTunnelRoutes(adapterName, overlayNet string, dnsSetting string) {
	applyMeshRoutes(adapterName)
	if overlayNet == "0.0.0.0/0" {
		applyExitTunnelRoutes(adapterName)
	}
	
	if dnsSetting == "mscale" {
		// Set Mscale DNS (100.64.0.1) as primary, fallback to Google DNS
		hiddenCommand("netsh", "interface", "ipv4", "set", "dns",
			"name="+adapterName, "static", "100.64.0.1", "primary").Run()
		hiddenCommand("netsh", "interface", "ipv4", "add", "dns",
			"name="+adapterName, "8.8.8.8", "index=2").Run()
	} else if dnsSetting == "google" {
		// User chose not to use MagicDNS, fallback to pure Google DNS
		hiddenCommand("netsh", "interface", "ipv4", "set", "dns",
			"name="+adapterName, "static", "8.8.8.8", "primary").Run()
		hiddenCommand("netsh", "interface", "ipv4", "add", "dns",
			"name="+adapterName, "8.8.4.4", "index=2").Run()
	} else if dnsSetting == "off" {
		// User chose to disable DNS routing entirely
		// Do nothing.
	} else if dnsSetting != "" {
		// Custom DNS IP provided by the UI
		hiddenCommand("netsh", "interface", "ipv4", "set", "dns",
			"name="+adapterName, "static", dnsSetting, "primary").Run()
	}

	// Blackhole all IPv6 traffic to prevent leaks (since we only route IPv4)
	hiddenCommand("netsh", "interface", "ipv6", "add", "route",
		"::/0", "interface="+adapterName, "metric=1", "store=active").Run()
}

func removeWindowsTunnelRoutes(adapterName, overlayNet string) {
	if overlayNet == "exit-via" {
		overlayNet = "0.0.0.0/0"
	}
	// Clean up the server host route
	hiddenCommand("powershell", "-NoProfile", "-Command",
		"route delete "+serverIP).Run()

	for _, prefix := range []string{"0.0.0.0/0", "0.0.0.0/1", "128.0.0.0/1"} {
		hiddenCommand("netsh", "interface", "ipv4", "delete", "route",
			"prefix="+prefix, "interface="+adapterName, "store=active").Run()
	}
	if overlayNet != "" && overlayNet != "0.0.0.0/0" {
		hiddenCommand("netsh", "interface", "ipv4", "delete", "route",
			overlayNet, "name="+adapterName, "store=active").Run()
	}

	// Clean up IPv6 blackhole route
	hiddenCommand("netsh", "interface", "ipv6", "delete", "route",
		"::/0", "interface="+adapterName, "store=active").Run()
}

// restoreWindowsInternetRoutes removes VPN routes and resets DNS/metrics on physical adapters.
func restoreWindowsInternetRoutes(adapterName, overlayNet string) {
	removeWindowsTunnelRoutes(adapterName, overlayNet)
	if adapterName != "" {
		hiddenCommand("netsh", "interface", "ipv4", "set", "dns",
			"name="+adapterName, "dhcp").Run()
	}
	go restorePhysicalAdapterMetrics()
}

func restorePhysicalAdapterMetrics() {
	hiddenCommand("powershell", "-NoProfile", "-Command",
		"Get-NetAdapter | Where-Object { $_.Status -eq 'Up' } | ForEach-Object { "+
			"Set-DnsClientServerAddress -InterfaceAlias $_.Name -ResetServerAddresses -ErrorAction SilentlyContinue; "+
			"Set-NetIPInterface -InterfaceAlias $_.Name -InterfaceMetric Automatic -ErrorAction SilentlyContinue }").Run()
}

// defaultRouteViaAdapter reports whether Windows default route uses the VPN adapter.
func defaultRouteViaAdapter(adapterName string) string {
	out, err := exec.Command("route", "print", "0.0.0.0").Output()
	if err != nil {
		return ""
	}
	idx := interfaceIndex(adapterName)
	if idx == "" {
		return "Routes: could not read adapter index"
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "0.0.0.0") && !strings.HasPrefix(line, "128.0.0.0") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 5 && fields[4] == idx {
			return "System default route: MScale VPN (traffic should exit via tunnel)"
		}
	}
	return "Warning: default route may still use Wi-Fi — reconnect as Administrator"
}

func interfaceIndex(adapterName string) string {
	out, err := hiddenCommand("netsh", "interface", "ipv4", "show", "interfaces").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, adapterName) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			if _, err := strconv.Atoi(fields[0]); err == nil {
				return fields[0]
			}
		}
	}
	return ""
}
