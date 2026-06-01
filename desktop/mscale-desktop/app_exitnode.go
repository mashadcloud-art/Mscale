package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"mscale.core/api"
	"mscale.core/wg"
)

func (a *App) getAPIClient() *api.Client {
	return api.NewClient(a.getSessionToken(), fmt.Sprintf("http://%s:%s", serverIP, serverPort), false, nil)
}

func (a *App) ListExitNodesJSON() string {
	return a.getAPIClient().FetchExitNodes()
}

func (a *App) WakeDevice(targetDeviceID string) string {
	payload := map[string]string{
		"target_device_id": targetDeviceID,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL("/api/devices/wake"), bytes.NewBuffer(body))
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

	return "Success: Wake request sent via mesh"
}

func (a *App) EnableAsExitNode(countryCode, label string) string {
	return a.SetShareAsExit(true, countryCode)
}

func (a *App) GetShareAsExit() bool {
	return a.sharingExit
}

// SetShareAsExit toggles Tailscale-style "run as exit node" on this PC.
func (a *App) SetShareAsExit(enabled bool, countryCode string) string {
	if enabled {
		if a.sharingExit && a.assignedIP != "" {
			return "Success: This device is already an exit node. Keep the app connected."
		}
		if a.overlayNet == "exit-via" || a.exitGatewayIP != "" {
			a.teardownTunnel()
		}
		return a.ConnectTunnel("exit-node", "", "mscale")
	}

	if a.sharingExit && a.deviceID != "" {
		_ = a.getAPIClient().DisableExitNode(a.deviceID)
	}
	a.sharingExit = false
	if a.assignedIP != "" {
		return a.DisconnectTunnel()
	}
	return "Success: Exit node disabled"
}

func (a *App) VerifyExitRouteJSON() string {
	if a.deviceID == "" {
		b, _ := json.Marshal(map[string]string{"error": "Connect to Mscale first, then run the test."})
		return string(b)
	}

	statusURL := apiURL("/api/exit-route/status?device_id=" + a.deviceID)
	req, err := http.NewRequest(http.MethodGet, statusURL, nil)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(b)
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": "Could not reach server — " + err.Error()})
		return string(b)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := json.Marshal(map[string]string{"error": parseAPIError(resp)})
		return string(b)
	}

	var status map[string]interface{}
	if err := json.Unmarshal(body, &status); err != nil {
		b, _ := json.Marshal(map[string]string{"error": "invalid status response"})
		return string(b)
	}

	connected, _ := status["connected"].(bool)
	if !connected {
		status["hint"] = "Switch to Exit Node mode and select an exit device, then connect again."
		out, _ := json.Marshal(status)
		return string(out)
	}

	testBody, _ := json.Marshal(map[string]string{"device_id": a.deviceID})
	testReq, err := http.NewRequest(http.MethodPost, apiURL("/api/exit-route/test"), bytes.NewBuffer(testBody))
	if err != nil {
		status["test_error"] = err.Error()
		out, _ := json.Marshal(status)
		return string(out)
	}
	testReq.Header.Set("Content-Type", "application/json")
	testResp, err := a.httpClient.Do(testReq)
	if err != nil {
		status["test_error"] = "Could not reach server — " + err.Error()
		out, _ := json.Marshal(status)
		return string(out)
	}
	testData, _ := io.ReadAll(testResp.Body)
	testResp.Body.Close()
	if testResp.StatusCode == http.StatusOK {
		var test map[string]interface{}
		if json.Unmarshal(testData, &test) == nil {
			for k, v := range test {
				status[k] = v
			}
			status["test_sent"] = true
		}
	} else {
		var testErr map[string]interface{}
		if json.Unmarshal(testData, &testErr) == nil {
			if msg, ok := testErr["error"].(string); ok && msg != "" {
				status["test_error"] = msg
			}
			if msg, ok := testErr["message"].(string); ok && msg != "" {
				status["test_error"] = msg
			}
		}
		if _, ok := status["test_error"]; !ok {
			status["test_error"] = parseAPIError(testResp)
		}
	}

	out, _ := json.Marshal(status)
	return string(out)
}

func (a *App) enableWindowsForwarding() {
	if a.adapterName == "" {
		return
	}

	hiddenCommand("powershell", "-NoProfile", "-Command",
		"Set-ItemProperty -Path 'HKLM:\\SYSTEM\\CurrentControlSet\\Services\\Tcpip\\Parameters' -Name 'IPEnableRouter' -Value 1 -ErrorAction SilentlyContinue").Run()
	hiddenCommand("powershell", "-NoProfile", "-Command",
		"Set-Service -Name RemoteAccess -StartupType Manual -ErrorAction SilentlyContinue; Start-Service -Name RemoteAccess -ErrorAction SilentlyContinue").Run()

	hiddenCommand("powershell", "-NoProfile", "-Command",
		"Set-NetIPInterface -InterfaceAlias '"+a.adapterName+"' -Forwarding Enabled -ErrorAction SilentlyContinue").Run()
	
	hiddenCommand("powershell", "-NoProfile", "-Command",
		"Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.Name -ne '"+a.adapterName+"' } | ForEach-Object { Set-NetIPInterface -InterfaceAlias $_.Name -Forwarding Enabled -AddressFamily IPv4 -ErrorAction SilentlyContinue }").Run()
	
	hiddenCommand("powershell", "-NoProfile", "-Command",
		"Remove-NetNat -Name 'MscaleExitNat' -Confirm:$false -ErrorAction SilentlyContinue; New-NetNat -Name 'MscaleExitNat' -InternalIPInterfaceAddressPrefix '100.64.0.0/10' -ErrorAction SilentlyContinue").Run()
}

func (a *App) resolveExitNodeOverlay(exitNodeID string) (string, string, error) {
	listJSON := a.ListExitNodesJSON()
	var list []exitNodeListItem
	if err := json.Unmarshal([]byte(listJSON), &list); err != nil {
		return "", "", err
	}
	for _, n := range list {
		if n.ID == exitNodeID {
			if n.OverlayIP == nil || *n.OverlayIP == "" {
				return "", n.DeviceName, fmt.Errorf("exit node %s is not online on the mesh yet", n.DeviceName)
			}
			if n.Status != "Online" {
				return "", n.DeviceName, fmt.Errorf("exit node %s is offline", n.DeviceName)
			}
			label := n.DeviceName
			if n.CountryCode != nil {
				label = *n.CountryCode + " — " + n.DeviceName
			}
			return *n.OverlayIP, label, nil
		}
	}
	return "", "", fmt.Errorf("exit node not found")
}

func (a *App) ConnectTunnel(mode string, exitNodeID string, dnsSetting string) string {
	me, err := a.fetchMe()
	if err != nil {
		return "Error: You are not logged in — " + err.Error()
	}
	if strings.TrimSpace(me.DisplayName) != "" {
		a.loggedInUser = me.DisplayName + " (" + me.Email + ")"
	} else if strings.TrimSpace(me.Email) != "" {
		a.loggedInUser = me.Email
	}

	if a.assignedIP != "" {
		a.teardownTunnel()
	}

	privateKey, err := loadOrCreateDesktopPrivateKey()
	if err != nil {
		return "Error: Could not load private key — " + err.Error()
	}
	pubKeyBytes := privateKey.PublicKey()
	publicKey := hex.EncodeToString(pubKeyBytes[:])

	apiClient := a.getAPIClient()

	deviceID, err := apiClient.RegisterDevice(a.deviceName, publicKey, mode, exitNodeID)
	if err != nil {
		return "Error: device registration failed — " + err.Error()
	}
	a.deviceID = deviceID

	if err := apiClient.UpdateDeviceKey(deviceID, publicKey); err != nil {
		return "Error: public key update failed — " + err.Error()
	}

	enroll, err := apiClient.EnrollDevice(deviceID)
	if err != nil {
		return "Error: device enrollment failed — " + err.Error()
	}

	overlayNet := "100.64.0.0/10"
	a.exitGatewayIP = ""
	a.activeExitNodeID = ""
	exitLabel := ""

	if mode == "exit-via" {
		if strings.TrimSpace(exitNodeID) == "" {
			return "Error: pick an exit node device"
		}

		if err := apiClient.ActivateExitRoute(deviceID, exitNodeID); err != nil {
			return "Error: exit routing on server failed — " + err.Error()
		}

		gw, label, err := a.resolveExitNodeOverlay(exitNodeID)
		if err != nil {
			apiClient.DeactivateExitRoute(deviceID)
			return "Error: " + err.Error()
		}
		a.exitGatewayIP = gw
		exitLabel = label
		overlayNet = exitInternetWG
		a.overlayNet = "exit-via"
		a.activeExitNodeID = exitNodeID
		a.sharingExit = false
	} else if mode == "exit-node" {
		a.overlayNet = overlayNet
		a.sharingExit = false // set true after server enable succeeds
	} else {
		a.overlayNet = overlayNet
		a.sharingExit = false
	}

	tunDevice, err := createMScaleTUN()
	if err != nil {
		if mode == "exit-via" {
			apiClient.DeactivateExitRoute(deviceID)
		}
		return "Error: VPN adapter failed — " + err.Error()
	}
	a.tunDevice = tunDevice

	adapterName, err := tunDevice.Name()
	if err != nil {
		a.tunDevice.Close()
		a.tunDevice = nil
		if mode == "exit-via" {
			apiClient.DeactivateExitRoute(deviceID)
		}
		return "Error: get adapter name failed"
	}
	a.adapterName = adapterName

	wgEngine := wg.NewEngine(privateKey, nil)
	a.wgEngine = wgEngine

	if err := wgEngine.Start(tunDevice, enroll.ServerKey, mode == "exit-via"); err != nil {
		a.teardownTunnel()
		return "Error: WireGuard up failed — " + err.Error()
	}

	hiddenCommand("netsh", "interface", "ipv4", "set", "address",
		"name="+adapterName, "static", enroll.OverlayIP, "255.192.0.0").Run()

	applyWindowsTunnelRoutes(adapterName, overlayNet, dnsSetting)
	if mode == "exit-via" {
		applyExitTunnelRoutes(adapterName)
		apiClient.EnsureExitRouteByKey(publicKey, enroll.OverlayIP, exitNodeID)
	}

	a.assignedIP = enroll.OverlayIP
	a.startHeartbeatLoop()

	routeNote := defaultRouteViaAdapter(adapterName)

	if mode == "exit-via" {
		return fmt.Sprintf(
			"Assigned IP: %s\nExit via: %s (hub routes via %s)\n%s\nLogged in as: %s",
			enroll.OverlayIP, exitLabel, a.exitGatewayIP, routeNote, a.loggedInUser,
		)
	}
	if mode == "exit-node" {
		if err := apiClient.EnableExitNode(deviceID, "US"); err != nil {
			a.teardownTunnel()
			return "Error: could not register as exit node — " + err.Error()
		}
		a.enableWindowsForwarding()
		a.sharingExit = true
		return fmt.Sprintf(
			"Assigned IP: %s\nExit node active — others can route through this PC\n%s\nLogged in as: %s",
			enroll.OverlayIP, routeNote, a.loggedInUser,
		)
	}
	return fmt.Sprintf(
		"Assigned IP: %s\nMode: %s\n%s\nLogged in as: %s",
		enroll.OverlayIP, mode, routeNote, a.loggedInUser,
	)
}

func (a *App) DisconnectTunnel() string {
	stats := a.getUsageStats()
	a.teardownTunnel()
	return "Disconnected - internet routes restored" + stats
}

func (a *App) ForceDisconnectTunnel() string {
	stats := a.getUsageStats()
	a.teardownTunnel()
	return "Force disconnected - if web still fails, run: ipconfig /renew" + stats
}

func (a *App) teardownTunnel() {
	a.stopHeartbeat()

	adapter := a.adapterName
	overlay := a.overlayNet
	exitGW := a.exitGatewayIP
	deviceID := a.deviceID
	sharing := a.sharingExit

	restoreWindowsInternetRoutes(adapter, overlay)

	hiddenCommand("powershell", "-NoProfile", "-Command",
		"Remove-NetNat -Name 'MscaleExitNat' -Confirm:$false -ErrorAction SilentlyContinue").Run()

	if a.wgEngine != nil {
		a.wgEngine.Stop()
		a.wgEngine = nil
	}
	if a.tunDevice != nil {
		a.tunDevice.Close()
		a.tunDevice = nil
	}

	a.exitGatewayIP = ""
	a.activeExitNodeID = ""
	a.adapterName = ""
	a.overlayNet = ""
	a.assignedIP = ""
	a.sharingExit = false
	a.deviceID = ""

	apiClient := a.getAPIClient()

	if sharing && deviceID != "" {
		go func() {
			_ = apiClient.DisableExitNode(deviceID)
		}()
	}

	if exitGW != "" && deviceID != "" {
		go func() {
			_ = apiClient.DeactivateExitRoute(deviceID)
		}()
	}

	if deviceID != "" {
		go apiClient.PostStatus(deviceID, "offline")
	}
}

func (a *App) GetStatus() string {
	if a.assignedIP == "" {
		if a.loggedInUser != "" {
			return "Logged in as: " + a.loggedInUser
		}
		return "Disconnected"
	}
	if a.sharingExit {
		return fmt.Sprintf("Exit node active — IP: %s | %s", a.assignedIP, a.loggedInUser)
	}
	return fmt.Sprintf("Connected — IP: %s | %s", a.assignedIP, a.loggedInUser)
}

func (a *App) getUsageStats() string {
	if a.wgEngine == nil || a.wgEngine.Device == nil {
		return ""
	}
	uapi, err := a.wgEngine.Device.IpcGet()
	if err != nil {
		return ""
	}
	
	var rxTotal, txTotal int64
	rxRe := regexp.MustCompile(`rx_bytes=(\d+)`)
	txRe := regexp.MustCompile(`tx_bytes=(\d+)`)
	
	for _, match := range rxRe.FindAllStringSubmatch(uapi, -1) {
		if val, err := strconv.ParseInt(match[1], 10, 64); err == nil {
			rxTotal += val
		}
	}
	for _, match := range txRe.FindAllStringSubmatch(uapi, -1) {
		if val, err := strconv.ParseInt(match[1], 10, 64); err == nil {
			txTotal += val
		}
	}
	
	if rxTotal == 0 && txTotal == 0 {
		return ""
	}
	
	formatBytes := func(b int64) string {
		const unit = 1024
		if b < unit {
			return fmt.Sprintf("%d B", b)
		}
		div, exp := int64(unit), 0
		for n := b / unit; n >= unit; n /= unit {
			div *= unit
			exp++
		}
		return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
	}
	
	return fmt.Sprintf(" \nSession Usage: Downloaded: %s | Uploaded: %s", formatBytes(rxTotal), formatBytes(txTotal))
}
