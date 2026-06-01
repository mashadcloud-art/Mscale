package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// EnrollPayload contains the mesh overlay IP and server public key returned by the API.
type EnrollPayload struct {
	OverlayIP string `json:"overlay_ip"`
	ServerKey string `json:"server_key"`
}

// RegisterDevice registers a new device or updates an existing one on the Mscale Hub.
func (c *Client) RegisterDevice(deviceName, publicKeyHex, tunnelMode, exitNodeID string) (string, error) {
	payload := map[string]string{
		"device_name": deviceName,
		"platform":    "android",
		"device_type": "mobile",
		"public_key":  publicKeyHex,
		"app_version": AppVersion,
		"os_version":  "android",
		"tunnel_mode": tunnelMode,
		"endpoint_ip": "129.151.146.44",
	}
	if exitNodeID != "" {
		payload["exit_node_id"] = exitNodeID
	}
	body, _ := json.Marshal(payload)
	resp, err := c.Call(http.MethodPost, "/api/devices/register", body)
	if err != nil {
		return "", fmt.Errorf("register failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("register: %s", ParseAPIError(resp))
	}
	
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.ID == "" {
		return "", fmt.Errorf("invalid register response")
	}
	return out.ID, nil
}

// UpdateDeviceKey pushes the WireGuard public key to the Mscale Hub.
func (c *Client) UpdateDeviceKey(deviceID, publicKeyHex string) error {
	payload := map[string]string{
		"device_id":  deviceID,
		"public_key": publicKeyHex,
	}
	body, _ := json.Marshal(payload)
	resp, err := c.Call(http.MethodPost, "/api/devices/update-key", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update-key: %s", ParseAPIError(resp))
	}
	return nil
}

// EnrollDevice gets the assigned Overlay IP and Server Key for the WireGuard tunnel.
func (c *Client) EnrollDevice(deviceID string) (*EnrollPayload, error) {
	payload := map[string]string{"device_id": deviceID}
	body, _ := json.Marshal(payload)
	resp, err := c.Call(http.MethodPost, "/api/devices/enroll", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("enroll: %s", ParseAPIError(resp))
	}
	
	var out EnrollPayload
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("invalid enroll response")
	}
	return &out, nil
}

// PostStatus sends a heartbeat status to the Mscale Hub.
func (c *Client) PostStatus(deviceID, status string) {
	if deviceID == "" {
		return
	}
	payload, _ := json.Marshal(map[string]string{"status": status, "peer_id": deviceID})
	resp, err := c.Call(http.MethodPost, "/status/update", payload)
	if err != nil {
		return
	}
	resp.Body.Close()
}
