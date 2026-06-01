package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type EnableExitNodeRequest struct {
	DeviceID    string `json:"device_id"`
	Label       string `json:"label"`
	CountryCode string `json:"country_code"`
	City        string `json:"city"`
	IsPrivate   *bool  `json:"is_private"`
}

type ExitNodeListItem struct {
	ID           string  `json:"id"`
	DeviceID     string  `json:"device_id"`
	DeviceName   string  `json:"device_name"`
	Label        string  `json:"label"`
	CountryCode  *string `json:"country_code,omitempty"`
	City         *string `json:"city,omitempty"`
	OverlayIP    *string `json:"overlay_ip,omitempty"`
	OwnerUserID  string  `json:"owner_user_id"`
	OwnerEmail   string  `json:"owner_email,omitempty"`
	OwnerName    string  `json:"owner_name,omitempty"`
	Status       string  `json:"status"`
	LastSeenAt   *string `json:"last_seen_at,omitempty"`
	IsPrivate    bool    `json:"is_private"`
	IsEnabled    bool    `json:"is_enabled"`
	HealthStatus string  `json:"health_status"`
}

func (h *AuthHandler) EnableExitNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req EnableExitNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.Label = strings.TrimSpace(req.Label)
	req.CountryCode = strings.ToUpper(strings.TrimSpace(req.CountryCode))
	req.City = strings.TrimSpace(req.City)

	if req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id is required"})
		return
	}
	if req.Label == "" {
		req.Label = "Exit Node"
	}

	var deviceName string
	err = h.DB.QueryRow(
		`SELECT device_name FROM devices WHERE id = ? AND user_id = ?`,
		req.DeviceID, session.UserID,
	).Scan(&deviceName)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	isPrivate := 1 // STRICT ISOLATION: always private

	var exitNodeID string
	err = h.DB.QueryRow(`SELECT id FROM exit_nodes WHERE device_id = ?`, req.DeviceID).Scan(&exitNodeID)
	if err == sql.ErrNoRows {
		exitNodeID = fmt.Sprintf("exit-%s-%s", req.DeviceID, time.Now().Format("20060102150405"))
		_, err = h.DB.Exec(
			`INSERT INTO exit_nodes (id, device_id, owner_user_id, label, country_code, city, is_private, is_enabled, health_status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, 1, 'healthy')`,
			exitNodeID, req.DeviceID, session.UserID, req.Label, nullIfEmpty(req.CountryCode), nullIfEmpty(req.City), isPrivate,
		)
	} else if err == nil {
		_, err = h.DB.Exec(
			`UPDATE exit_nodes SET label = ?, country_code = ?, city = ?, is_private = ?, is_enabled = 1, health_status = 'healthy'
			 WHERE device_id = ? AND owner_user_id = ?`,
			req.Label, nullIfEmpty(req.CountryCode), nullIfEmpty(req.City), isPrivate, req.DeviceID, session.UserID,
		)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not enable exit node"})
		return
	}

	_, _ = h.DB.Exec(`UPDATE devices SET exit_node_id = ? WHERE id = ? AND user_id = ?`, exitNodeID, req.DeviceID, session.UserID)

	var overlayIP, pubKey string
	_ = h.DB.QueryRow(
		`SELECT overlay_ip, public_key FROM devices WHERE id = ? AND user_id = ?`,
		req.DeviceID, session.UserID,
	).Scan(&overlayIP, &pubKey)
	if overlayIP != "" && pubKey != "" {
		_ = syncExitNodePeer(overlayIP, pubKey)
	}

	NotifyDevicesChanged()
	writeJSON(w, http.StatusOK, map[string]string{
		"message":      "device is now an exit node",
		"exit_node_id": exitNodeID,
		"device_name":  deviceName,
	})
}

func (h *AuthHandler) DisableExitNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	session, err := h.GetSession(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id is required"})
		return
	}

	_, err = h.DB.Exec(
		`UPDATE exit_nodes SET is_enabled = 0, health_status = 'disabled'
		 WHERE device_id = ? AND owner_user_id = ?`,
		req.DeviceID, session.UserID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	_, _ = h.DB.Exec(`UPDATE devices SET exit_node_id = NULL WHERE id = ? AND user_id = ?`, req.DeviceID, session.UserID)

	NotifyDevicesChanged()
	writeJSON(w, http.StatusOK, map[string]string{"message": "exit node disabled"})
}

func (h *AuthHandler) ListExitNodes(w http.ResponseWriter, r *http.Request) {
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
			en.id, en.device_id, d.device_name, en.label, en.country_code, en.city,
			d.overlay_ip, en.owner_user_id, u.email, u.display_name,
			d.status, d.last_seen_at, en.is_private, en.is_enabled, en.health_status
		FROM exit_nodes en
		INNER JOIN devices d ON d.id = en.device_id
		INNER JOIN users u ON u.id = en.owner_user_id
		WHERE en.is_enabled = 1
		  AND d.device_type IN ('desktop', 'server', 'mobile')
		  AND en.owner_user_id = ?
		ORDER BY en.country_code, d.device_name
	`, session.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	list := []ExitNodeListItem{}
	for rows.Next() {
		var item ExitNodeListItem
		var countryCode, city, overlayIP sql.NullString
		var ownerEmail, ownerName string
		var lastSeenAt sql.NullTime
		var isPrivate int

		err := rows.Scan(
			&item.ID, &item.DeviceID, &item.DeviceName, &item.Label,
			&countryCode, &city, &overlayIP, &item.OwnerUserID, &ownerEmail, &ownerName,
			&item.Status, &lastSeenAt, &isPrivate, &item.IsEnabled, &item.HealthStatus,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read exit nodes"})
			return
		}

		item.IsPrivate = isPrivate == 1
		item.OwnerEmail = ownerEmail
		item.OwnerName = ownerName
		if countryCode.Valid {
			item.CountryCode = &countryCode.String
		}
		if city.Valid {
			item.City = &city.String
		}
		if overlayIP.Valid {
			item.OverlayIP = &overlayIP.String
		}
		if lastSeenAt.Valid {
			s := lastSeenAt.Time.UTC().Format(time.RFC3339)
			item.LastSeenAt = &s
			item.Status = DeriveDeviceStatus(item.Status, lastSeenAt)
		} else {
			item.Status = "Offline"
		}

		list = append(list, item)
	}

	if list == nil {
		list = []ExitNodeListItem{}
	}
	isHubExit := GetSetting(h.DB, "hub_is_exit_node", "false")
	// Only offer hub exit when no phone/PC exit nodes exist — hub exit uses Oracle IP, not mobile.
	if isHubExit == "true" && len(list) == 0 {
		hubCountry := "IN"
		list = append(list, ExitNodeListItem{
			ID:          "hub",
			DeviceID:    "hub-device",
			DeviceName:  "Oracle Cloud Hub",
			Label:       "Oracle Cloud Hub",
			CountryCode: &hubCountry,
			OwnerUserID: session.UserID,
			Status:      "Online",
			IsEnabled:   true,
			IsPrivate:   false,
		})
	}

	writeJSON(w, http.StatusOK, list)
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// GetExitNodeDevice returns overlay IP and owner device info for routing client traffic.
func (h *AuthHandler) GetExitNodeForConnect(exitNodeID, userID string) (overlayIP, deviceName, country string, err error) {
	var ownerID string
	var oIP sql.NullString
	var cc sql.NullString

	err = h.DB.QueryRow(`
		SELECT d.overlay_ip, d.device_name, en.country_code, en.owner_user_id
		FROM exit_nodes en
		INNER JOIN devices d ON d.id = en.device_id
		WHERE en.id = ? AND en.is_enabled = 1
	`, exitNodeID).Scan(&oIP, &deviceName, &cc, &ownerID)
	if err == sql.ErrNoRows {
		return "", "", "", fmt.Errorf("exit node not found")
	}
	if err != nil {
		return "", "", "", err
	}

	if !oIP.Valid || oIP.String == "" {
		return "", "", "", fmt.Errorf("exit node device is not enrolled on the mesh yet — connect it once first")
	}

	if ownerID != userID {
		return "", "", "", fmt.Errorf("you do not have access to this exit node")
	}

	if cc.Valid {
		country = cc.String
	}
	return oIP.String, deviceName, country, nil
}
