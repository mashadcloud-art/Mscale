package api

import (
	"database/sql"
	"net/http"
	"time"
)

type MeDeviceResponse struct {
	Device   DeviceListItem `json:"device"`
	ExitNode *ExitNodeSummary `json:"exit_node,omitempty"`
}

func (h *AuthHandler) MeDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

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

	err = h.DB.QueryRow(`
		SELECT
			d.id, d.user_id, d.device_name, d.platform, d.device_type, d.public_key,
			d.overlay_ip, d.app_version, d.os_version, d.current_dns, d.exit_node_id,
			d.tunnel_mode, d.endpoint_ip, d.enrolled_at, d.last_seen_at, d.status,
			en.id, en.device_id, en.owner_user_id, en.label, en.country_code, en.city,
			en.health_status, en.is_private, en.is_enabled
		FROM devices d
		LEFT JOIN exit_nodes en ON d.exit_node_id = en.id
		WHERE d.user_id = ? AND d.status = 'online'
		LIMIT 1
	`, session.UserID).Scan(
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

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no active device found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	d.EnrolledAt = enrolledAt.Format(time.RFC3339)
	if lastSeenAt.Valid {
		s := lastSeenAt.Time.Format(time.RFC3339)
		d.LastSeenAt = &s
	}

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

	var exitNode *ExitNodeSummary
	if enID.Valid {
		exitNode = &ExitNodeSummary{
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
	}
	d.ExitNode = exitNode

	writeJSON(w, http.StatusOK, MeDeviceResponse{
		Device:   d,
		ExitNode: d.ExitNode,
	})
}

