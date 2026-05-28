package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		AppVersion: "0.1.0",
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
	me, err := a.fetchMe()
	if err != nil {
		return "Error: login required — " + err.Error()
	}
	if strings.TrimSpace(me.DisplayName) != "" {
		a.loggedInUser = me.DisplayName + " (" + me.Email + ")"
	}

	if a.assignedIP == "" {
		if msg := a.ConnectTunnel("mesh", ""); strings.HasPrefix(msg, "Error") {
			return msg
		}
	}

	if a.deviceID == "" {
		return "Error: connect to mesh first before offering exit node"
	}

	countryCode = strings.ToUpper(strings.TrimSpace(countryCode))
	label = strings.TrimSpace(label)
	if label == "" {
		label = a.deviceName + " exit"
	}

	payload := map[string]interface{}{
		"device_id":    a.deviceID,
		"label":        label,
		"country_code": countryCode,
		"is_private":   false,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", apiURL("/api/devices/exit-node/enable"), bytes.NewBuffer(body))
	if err != nil {
		return "Error: could not build request"
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "Error: could not reach server — " + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "Error: " + parseAPIError(resp)
	}

	a.enableWindowsForwarding()
	return fmt.Sprintf("Success: This device is now an exit node (%s). Keep the app connected.", countryCode)
}

func (a *App) enableWindowsForwarding() {
	if a.adapterName == "" {
		return
	}
	hiddenCommand("powershell", "-NoProfile", "-Command",
		"Set-NetIPInterface -InterfaceAlias '"+a.adapterName+"' -Forwarding Enabled -ErrorAction SilentlyContinue").Run()
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

func (a *App) ConnectTunnel(mode string, exitNodeID string) string {
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
	} else {
		a.overlayNet = overlayNet
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

	applyMeshRoutes(adapterName)
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
	return fmt.Sprintf(
		"Assigned IP: %s\nMode: %s\n%s\nLogged in as: %s",
		enroll.OverlayIP, mode, routeNote, a.loggedInUser,
	)
}

func (a *App) DisconnectTunnel() string {
	a.teardownTunnel()
	return "Disconnected — internet routes restored"
}

func (a *App) ForceDisconnectTunnel() string {
	a.teardownTunnel()
	return "Force disconnected — if web still fails, run: ipconfig /renew"
}

func (a *App) teardownTunnel() {
	a.stopHeartbeat()

	adapter := a.adapterName
	overlay := a.overlayNet
	exitGW := a.exitGatewayIP
	deviceID := a.deviceID

	restoreWindowsInternetRoutes(adapter, overlay)

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
	a.deviceID = ""

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
	return fmt.Sprintf("Connected — IP: %s | %s", a.assignedIP, a.loggedInUser)
}
