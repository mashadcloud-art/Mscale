package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

type appRelease struct {
	Version        string `json:"version"`
	DownloadURL    string `json:"download_url"`
	SetupURL       string `json:"setup_url"`
	AndroidVersion string `json:"android_version"`
	AndroidAPKURL  string `json:"android_apk_url"`
	Notes          string `json:"notes"`
	Required       bool   `json:"required"`
}

func loadAppRelease() appRelease {
	paths := []string{"releases/latest.json", "latest.json"}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var rel appRelease
		if err := json.Unmarshal(raw, &rel); err != nil {
			continue
		}
		if strings.TrimSpace(rel.Version) != "" {
			return rel
		}
	}
	return appRelease{
		Version:     "0.4.26",
		DownloadURL: "https://mashad.shop/mscale/downloads/mscale-desktop-v0.4.26.exe",
		SetupURL:    "https://mashad.shop/mscale/downloads/MscaleSetup-v0.4.26.exe",
		Notes:       "Latest stable release.",
	}
}

// LatestAppRelease returns the current desktop release manifest for update checks.
func LatestAppRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	rel := loadAppRelease()
	writeJSON(w, http.StatusOK, rel)
}
