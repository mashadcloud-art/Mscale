package api

import (
	"database/sql"
	"strings"
	"time"
)

// DeriveDeviceStatus maps DB status + last heartbeat to admin UI status.
// Explicit offline in the DB always wins (e.g. desktop disconnect).
func DeriveDeviceStatus(dbStatus string, lastSeenAt sql.NullTime) string {
	if strings.EqualFold(strings.TrimSpace(dbStatus), "offline") {
		return "Offline"
	}
	if lastSeenAt.Valid {
		if time.Since(lastSeenAt.Time) > 2*time.Minute {
			return "Offline"
		}
		return "Online"
	}
	return "Offline"
}

// NormalizeDeviceDBStatus stores a consistent lowercase status in SQLite.
func NormalizeDeviceDBStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "offline":
		return "offline"
	case "active", "online", "":
		return "online"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

// DevicesChanged is set from main to push live updates (WebSocket hub).
var DevicesChanged func()

func NotifyDevicesChanged() {
	if DevicesChanged != nil {
		DevicesChanged()
	}
}

// DedupeDevicesByName keeps one row per device name (case-insensitive), preferring the newest activity.
func DedupeDevicesByName(devices []DeviceListItem) []DeviceListItem {
	type entry struct {
		item DeviceListItem
		sort time.Time
	}
	byName := make(map[string]entry)

	for _, d := range devices {
		key := strings.ToLower(strings.TrimSpace(d.DeviceName))
		if key == "" {
			key = d.ID
		}
		sortKey := time.Time{}
		if d.LastSeenAt != nil {
			if t, err := time.Parse(time.RFC3339, *d.LastSeenAt); err == nil {
				sortKey = t
			}
		}
		if sortKey.IsZero() {
			if t, err := time.Parse(time.RFC3339, d.EnrolledAt); err == nil {
				sortKey = t
			}
		}
		if prev, ok := byName[key]; !ok || sortKey.After(prev.sort) {
			byName[key] = entry{item: d, sort: sortKey}
		}
	}

	out := make([]DeviceListItem, 0, len(byName))
	for _, e := range byName {
		out = append(out, e.item)
	}
	return out
}
