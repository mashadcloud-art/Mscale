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
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func (a *App) ensureDeviceRecord(mode string, publicKey string) (string, error) {
	payload := deviceRegisterRequest{
		DeviceName: a.deviceName,
		Platform:   "windows",
		DeviceType: "desktop",
		PublicKey:  publicKey,
		AppVersion: GetAppVersion(),
		OSVersion:  "windows",
		CurrentDNS: getSystemDNS(),
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

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		body, _ := io.ReadAll(resp.Body)
		var out deviceRegisterResponse
		if err := json.Unmarshal(body, &out); err != nil {
			return "", fmt.Errorf("invalid register response: %w", err)
		}
		if out.ID == "" {
			return "", fmt.Errorf("register response missing device id")
		}
		saveDeviceID(a.deviceName, out.ID)
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

func (a *App) ListExitNodesJSON() string {
	req, err := http.NewRequest("GET", apiURL("/api/exit-nodes"), nil)
	if err != nil {
		return "[]"
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "[]"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg := parseAPIError(resp)
		b, _ := json.Marshal([]map[string]string{{"_error": msg}})
		return string(b)
	}
	body, _ := io.ReadAll(resp.Body)
	return string(body)
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
		_ = a.disableExitNodeOnServer(a.deviceID)
	}
	a.sharingExit = false
	if a.assignedIP != "" {
		return a.DisconnectTunnel()
	}
	return "Success: Exit node disabled"
}

func (a *App) disableExitNodeOnServer(deviceID string) error {
	payload := map[string]string{"device_id": deviceID}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", apiURL("/api/devices/exit-node/disable"), bytes.NewBuffer(body))
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

func (a *App) enableExitNodeOnServer(deviceID, countryCode, label string) error {
	countryCode = strings.ToUpper(strings.TrimSpace(countryCode))
	label = strings.TrimSpace(label)
	if countryCode == "" {
		countryCode = "US"
	}
	if label == "" {
		label = a.deviceName + " exit"
	}
	payload := map[string]interface{}{
		"device_id":    deviceID,
		"label":        label,
		"country_code": countryCode,
		"is_private":   false,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", apiURL("/api/devices/exit-node/enable"), bytes.NewBuffer(body))
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
	
	// 1. Enable IP forwarding on the WireGuard adapter
	hiddenCommand("powershell", "-NoProfile", "-Command",
		"Set-NetIPInterface -InterfaceAlias '"+a.adapterName+"' -Forwarding Enabled -ErrorAction SilentlyContinue").Run()
	
	// 2. Enable IP forwarding on all active physical adapters (Wi-Fi, Ethernet) so traffic can leave the machine
	hiddenCommand("powershell", "-NoProfile", "-Command",
		"Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.Name -ne '"+a.adapterName+"' } | ForEach-Object { Set-NetIPInterface -InterfaceAlias $_.Name -Forwarding Enabled -AddressFamily IPv4 -ErrorAction SilentlyContinue }").Run()
	
	// 3. Create NAT to masquerade WireGuard traffic to the physical adapter
	hiddenCommand("powershell", "-NoProfile", "-Command",
		"New-NetNat -Name 'MscaleExitNat' -InternalIPInterfaceAddressPrefix '100.64.0.0/10' -ErrorAction SilentlyContinue").Run()
}

func (a *App) activateExitRoute(exitNodeID string) error {
	if a.deviceID == "" {
		return fmt.Errorf("device not registered")
	}
	payload, _ := json.Marshal(map[string]string{
		"device_id":    a.deviceID,
		"exit_node_id": exitNodeID,
	})
	req, err := http.NewRequest("POST", apiURL("/api/exit-route/activate"), bytes.NewBuffer(payload))
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

func (a *App) ensureExitRouteByKey(publicKeyHex, overlayIP string) {
	if publicKeyHex == "" || overlayIP == "" {
		return
	}
	body, _ := json.Marshal(map[string]string{
		"public_key":  publicKeyHex,
		"overlay_ip":  overlayIP,
	})
	req, err := http.NewRequest("POST", apiURL("/api/exit-route/ensure-by-key"), bytes.NewBuffer(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func (a *App) deactivateExitRoute() {
	if a.deviceID == "" {
		return
	}
	payload, _ := json.Marshal(map[string]string{"device_id": a.deviceID})
	req, err := http.NewRequest("POST", apiURL("/api/exit-route/deactivate"), bytes.NewBuffer(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
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
	a.exitGatewayIP = ""
	exitLabel := ""

	if mode == "exit-via" {
		if strings.TrimSpace(exitNodeID) == "" {
			return "Error: pick an exit node device"
		}

		if err := a.activateExitRoute(exitNodeID); err != nil {
			return "Error: exit routing on server failed — " + err.Error()
		}

		gw, label, err := a.resolveExitNodeOverlay(exitNodeID)
		if err != nil {
			a.deactivateExitRoute()
			return "Error: " + err.Error()
		}
		a.exitGatewayIP = gw
		exitLabel = label
		overlayNet = exitInternetWG
		a.overlayNet = "exit-via"
		a.sharingExit = false
	} else if mode == "exit-node" {
		a.overlayNet = overlayNet
		a.sharingExit = false // set true after server enable succeeds
	} else {
		a.overlayNet = overlayNet
		a.sharingExit = false
	}

	serverKey, err := wgtypes.ParseKey(strings.TrimSpace(enroll.ServerKey))
	if err != nil {
		if mode == "exit-via" {
			a.deactivateExitRoute()
		}
		return "Error: Invalid server key"
	}

	tunDevice, err := createMScaleTUN()
	if err != nil {
		if mode == "exit-via" {
			a.deactivateExitRoute()
		}
		return "Error: VPN adapter failed — " + err.Error()
	}
	a.tunDevice = tunDevice

	adapterName, err := tunDevice.Name()
	if err != nil {
		a.tunDevice.Close()
		a.tunDevice = nil
		if mode == "exit-via" {
			a.deactivateExitRoute()
		}
		return "Error: get adapter name failed"
	}
	a.adapterName = adapterName

	logger := device.NewLogger(device.LogLevelError, "[WG] ")
	wgEngine := device.NewDevice(tunDevice, conn.NewDefaultBind(), logger)
	a.wgEngine = wgEngine

	wgEndpoint := serverIP + ":51820"
	var wgConfig string
	if mode == "exit-via" {
		wgConfig = fmt.Sprintf(
			"private_key=%s\npublic_key=%s\nendpoint=%s\nallowed_ip=0.0.0.0/1\nallowed_ip=128.0.0.0/1\npersistent_keepalive_interval=25\n",
			hex.EncodeToString(privateKey[:]),
			hex.EncodeToString(serverKey[:]),
			wgEndpoint,
		)
	} else {
		wgConfig = fmt.Sprintf(
			"private_key=%s\npublic_key=%s\nendpoint=%s\nallowed_ip=%s\npersistent_keepalive_interval=25\n",
			hex.EncodeToString(privateKey[:]),
			hex.EncodeToString(serverKey[:]),
			wgEndpoint,
			overlayNet,
		)
	}

	if err := wgEngine.IpcSet(wgConfig); err != nil {
		a.teardownTunnel()
		return "Error: WireGuard config failed — " + err.Error()
	}
	if err := wgEngine.Up(); err != nil {
		a.teardownTunnel()
		return "Error: WireGuard up failed — " + err.Error()
	}

	hiddenCommand("netsh", "interface", "ipv4", "set", "address",
		"name="+adapterName, "static", enroll.OverlayIP, "255.192.0.0").Run()

	applyWindowsTunnelRoutes(adapterName, overlayNet, dnsSetting)
	if mode == "exit-via" {
		applyExitTunnelRoutes(adapterName)
		a.ensureExitRouteByKey(publicKey, enroll.OverlayIP)
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
		if err := a.enableExitNodeOnServer(deviceID, "", ""); err != nil {
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

	// Clean up NAT
	hiddenCommand("powershell", "-NoProfile", "-Command",
		"Remove-NetNat -Name 'MscaleExitNat' -Confirm:$false -ErrorAction SilentlyContinue").Run()

	if a.wgEngine != nil {
		a.wgEngine.Down()
		a.wgEngine.Close()
		a.wgEngine = nil
	}
	if a.tunDevice != nil {
		a.tunDevice.Close()
		a.tunDevice = nil
	}

	a.exitGatewayIP = ""
	a.adapterName = ""
	a.overlayNet = ""
	a.assignedIP = ""
	a.sharingExit = false
	a.deviceID = ""

	if sharing && deviceID != "" {
		id := deviceID
		go func() {
			_ = a.disableExitNodeOnServer(id)
		}()
	}

	if exitGW != "" && deviceID != "" {
		id := deviceID
		go func() {
			a.deviceID = id
			a.deactivateExitRoute()
			a.deviceID = ""
		}()
	}

	if deviceID != "" {
		id := deviceID
		go a.postOfflineStatus(id)
	}
}

func (a *App) postOfflineStatus(deviceID string) {
	payload := map[string]string{"status": "offline", "peer_id": deviceID}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", apiURL("/status/update"), bytes.NewBuffer(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 4 * time.Second}
	if resp, err := client.Do(req); err == nil {
		resp.Body.Close()
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
	if a.wgEngine == nil {
		return ""
	}
	uapi, err := a.wgEngine.IpcGet()
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

