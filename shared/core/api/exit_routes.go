package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ActivateExitRoute sets the exit_node_id on the server for this device.
func (c *Client) ActivateExitRoute(deviceID, exitNodeID string) error {
	payload := map[string]string{
		"device_id":    deviceID,
		"exit_node_id": exitNodeID,
	}
	body, _ := json.Marshal(payload)
	resp, err := c.Call(http.MethodPost, "/api/exit-route/activate", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("exit route: %s", ParseAPIError(resp))
	}
	return nil
}

// EnsureExitRouteByKey registers the exit route by public key directly (used for WireGuard sync).
func (c *Client) EnsureExitRouteByKey(publicKeyHex, overlayIP, exitNodeID string) {
	if publicKeyHex == "" || overlayIP == "" {
		return
	}
	payload := map[string]string{
		"public_key": publicKeyHex,
		"overlay_ip": overlayIP,
	}
	if exitNodeID != "" {
		payload["exit_node_id"] = exitNodeID
	}
	body, _ := json.Marshal(payload)
	resp, err := c.Call(http.MethodPost, "/api/exit-route/ensure-by-key", body)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// DeactivateExitRoute clears the exit_node_id for this device.
func (c *Client) DeactivateExitRoute(deviceID string) error {
	payload := map[string]string{"device_id": deviceID}
	body, _ := json.Marshal(payload)
	resp, err := c.Call(http.MethodPost, "/api/exit-route/deactivate", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// EnableExitNode marks this device as an exit node provider.
func (c *Client) EnableExitNode(deviceID, countryCode string) error {
	label := countryCode + " mobile exit"
	payload := map[string]interface{}{
		"device_id":    deviceID,
		"label":        label,
		"country_code": countryCode,
		"is_private":   false,
	}
	body, _ := json.Marshal(payload)
	resp, err := c.Call(http.MethodPost, "/api/devices/exit-node/enable", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("enable exit: %s", ParseAPIError(resp))
	}
	return nil
}

// DisableExitNode unmarks this device as an exit node.
func (c *Client) DisableExitNode(deviceID string) error {
	payload := map[string]string{"device_id": deviceID}
	body, _ := json.Marshal(payload)
	resp, err := c.Call(http.MethodPost, "/api/devices/exit-node/disable", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// FetchExitNodes retrieves the list of available exit nodes.
func (c *Client) FetchExitNodes() string {
	resp, err := c.Call(http.MethodGet, "/api/exit-nodes", nil)
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
