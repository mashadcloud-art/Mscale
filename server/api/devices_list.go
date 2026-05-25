package api

import (
	"database/sql"
	"net/http"
	"time"
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
	Status     string  `json:"status"`
	ExitNode   *ExitNodeSummary `json:"exit_node,omitempty"`
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

	rows, err := h.DB.Query(`
		SELECT
			d.id, d.user_id, d.device_name, d.platform, d.device_type, d.public_key,
			d.overlay_ip, d.app_version, d.os_version, d.current_dns, d.exit_node_id,
			d.tunnel_mode, d.endpoint_ip, d.enrolled_at, d.last_seen_at, d.status,
			en.id, en.device_id, en.owner_user_id, en.label, en.country_code, en.city,
			en.health_status, en.is_private, en.is_enabled
		FROM devices d
		LEFT JOIN exit_nodes en ON d.exit_node_id = en.id
		WHERE d.user_id = ?
		ORDER BY d.enrolled_at DESC
	`, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	devices := []DeviceListItem{}

	for rows.Next() {
		var d DeviceListItem
		var enrolledAt time.Time
		var lastSeenAt sql.NullTime
		var overlayIP sql.NullString
		var appVersion sql.NullString
		var osVersion sql.NullString
		var currentDNS sql.NullString
		var exitNodeID sql.NullString
		var tunnelMode sql.NullString
		var endpointIP sql.NullString

		var enID sql.NullString
		var enDeviceID sql.NullString
		var enOwnerUserID sql.NullString
		var enLabel sql.NullString
		var enCountryCode sql.NullString
		var enCity sql.NullString
		var enHealthStatus sql.NullString
		var enIsPrivate sql.NullInt64
		var enIsEnabled sql.NullInt64

		err := rows.Scan(
			&d.ID,
			&d.UserID,
			&d.DeviceName,
			&d.Platform,
			&d.DeviceType,
			&d.PublicKey,
			&overlayIP,
			&appVersion,
			&osVersion,
			&currentDNS,
			&exitNodeID,
			&tunnelMode,
			&endpointIP,
			&enrolledAt,
			&lastSeenAt,
			&d.Status,
			&enID,
			&enDeviceID,
			&enOwnerUserID,
			&enLabel,
			&enCountryCode,
			&enCity,
			&enHealthStatus,
			&enIsPrivate,
			&enIsEnabled,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read devices"})
			return
		}

		d.EnrolledAt = enrolledAt.Format(time.RFC3339)

		if overlayIP.Valid {
			d.OverlayIP = &overlayIP.String
		}
		if appVersion.Valid {
			d.AppVersion = &appVersion.String
		}
		if osVersion.Valid {
			d.OSVersion = &osVersion.String
		}
		if currentDNS.Valid {
			d.CurrentDNS = &currentDNS.String
		}
		if exitNodeID.Valid {
			d.ExitNodeID = &exitNodeID.String
		}
		if tunnelMode.Valid {
			d.TunnelMode = &tunnelMode.String
		}
		if endpointIP.Valid {
			d.EndpointIP = &endpointIP.String
		}
		if lastSeenAt.Valid {
			s := lastSeenAt.Time.Format(time.RFC3339)
			d.LastSeenAt = &s
			
			// Dynamically determine status based on heartbeat
			if time.Since(lastSeenAt.Time) > 2*time.Minute {
				d.Status = "Offline"
			} else {
				d.Status = "Online"
			}
		} else {
			d.Status = "Offline"
		}

		if enID.Valid {
			exitNode := ExitNodeSummary{
				ID:               enID.String,
				Label:            enLabel.String,
				HealthStatus:     enHealthStatus.String,
				IsPrivate:        enIsPrivate.Valid && enIsPrivate.Int64 == 1,
				IsEnabled:        enIsEnabled.Valid && enIsEnabled.Int64 == 1,
				OwnerUserID:      enOwnerUserID.String,
				ExitNodeDeviceID: enDeviceID.String,
			}
			if enCountryCode.Valid {
				exitNode.CountryCode = &enCountryCode.String
			}
			if enCity.Valid {
				exitNode.City = &enCity.String
			}
			d.ExitNode = &exitNode
		}

		devices = append(devices, d)
	}

	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	writeJSON(w, http.StatusOK, devices)
}

