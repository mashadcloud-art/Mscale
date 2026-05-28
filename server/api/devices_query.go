package api

import (
	"database/sql"
	"strings"
	"time"
)

// ListDesktopDevicesForUser returns real desktop-app devices for one user (admin panel + WebSocket).
func ListDesktopDevicesForUser(db *sql.DB, userID string) ([]DeviceListItem, error) {
	rows, err := db.Query(`
		SELECT
			d.id, d.user_id, d.device_name, d.platform, d.device_type, d.public_key,
			d.overlay_ip, d.app_version, d.os_version, d.current_dns, d.exit_node_id,
			d.tunnel_mode, d.endpoint_ip, d.enrolled_at, d.last_seen_at, d.status,
			u.email, u.display_name,
			en.id, en.device_id, en.owner_user_id, en.label, en.country_code, en.city,
			en.health_status, en.is_private, en.is_enabled
		FROM devices d
		INNER JOIN users u ON u.id = d.user_id
		LEFT JOIN exit_nodes en ON d.exit_node_id = en.id
		WHERE d.user_id = ?
		  AND d.device_type = 'desktop'
		  AND LOWER(d.device_name) NOT LIKE 'test-%'
		ORDER BY d.device_name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := []DeviceListItem{}

	for rows.Next() {
		var d DeviceListItem
		var userEmail, userDisplayName string
		var enrolledAt time.Time
		var lastSeenAt sql.NullTime
		var overlayIP, appVersion, osVersion, currentDNS, exitNodeID, tunnelMode, endpointIP sql.NullString

		var enID, enDeviceID, enOwnerUserID, enLabel sql.NullString
		var enCountryCode, enCity, enHealthStatus sql.NullString
		var enIsPrivate, enIsEnabled sql.NullInt64

		err := rows.Scan(
			&d.ID, &d.UserID, &d.DeviceName, &d.Platform, &d.DeviceType, &d.PublicKey,
			&overlayIP, &appVersion, &osVersion, &currentDNS, &exitNodeID,
			&tunnelMode, &endpointIP, &enrolledAt, &lastSeenAt, &d.Status,
			&userEmail, &userDisplayName,
			&enID, &enDeviceID, &enOwnerUserID, &enLabel, &enCountryCode, &enCity,
			&enHealthStatus, &enIsPrivate, &enIsEnabled,
		)
		if err != nil {
			return nil, err
		}

		d.OwnerEmail = userEmail
		if strings.TrimSpace(userDisplayName) != "" {
			d.OwnerName = userDisplayName
		} else {
			d.OwnerName = userEmail
		}

		d.EnrolledAt = enrolledAt.UTC().Format(time.RFC3339)
		if overlayIP.Valid {
			d.OverlayIP = &overlayIP.String
		}
		if appVersion.Valid {
			d.AppVersion = &appVersion.String
		}
		if osVersion.Valid {
			d.OSVersion = &osVersion.String
		}
		if currentDNS.Valid && currentDNS.String != "" {
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
			s := lastSeenAt.Time.UTC().Format(time.RFC3339)
			d.LastSeenAt = &s
		}
		d.Status = DeriveDeviceStatus(d.Status, lastSeenAt)

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

	return DedupeDevicesByName(devices), rows.Err()
}
