package api

import (
	"net/http"
)

type ExitNodeSummary struct {
	ID              string  `json:"id"`
	Label           string  `json:"label"`
	CountryCode     *string `json:"country_code,omitempty"`
	City            *string `json:"city,omitempty"`
	HealthStatus    string  `json:"health_status"`
	IsPrivate       bool    `json:"is_private"`
	IsEnabled       bool    `json:"is_enabled"`
	OwnerUserID     string  `json:"owner_user_id"`
	ExitNodeDeviceID string `json:"device_id"`
}

type DeviceListItem struct {
	ID         string  `json:"id"`
	UserID     string  `json:"user_id"`
	OwnerEmail string  `json:"owner_email,omitempty"`
	OwnerName  string  `json:"owner_name,omitempty"`
	DeviceName string  `json:"device_name"`
	Platform   string  `json:"platform"`
	DeviceType string  `json:"device_type"`
	PublicKey  string  `json:"public_key"`
	OverlayIP  *string `json:"overlay_ip,omitempty"`
	AppVersion *string `json:"app_version,omitempty"`
	OSVersion  *string `json:"os_version,omitempty"`
	CurrentDNS *string `json:"current_dns,omitempty"`
	ExitNodeID *string `json:"exit_node_id,omitempty"`
	TunnelMode *string `json:"tunnel_mode,omitempty"`
	EndpointIP *string `json:"endpoint_ip,omitempty"`
	EnrolledAt string  `json:"enrolled_at"`
	LastSeenAt *string `json:"last_seen_at,omitempty"`
	Status            string  `json:"status"`
	WakeRemoteEnabled bool    `json:"wake_remote_enabled"`
	WakeCallNumbers   string  `json:"wake_call_numbers,omitempty"`
	DevicePhone       string  `json:"device_phone,omitempty"`
	ExitNode          *ExitNodeSummary `json:"exit_node,omitempty"`
}

func (h *AuthHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	devices, err := ListDesktopDevicesForUser(h.DB, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	writeJSON(w, http.StatusOK, devices)
}

