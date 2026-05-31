package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type releaseInfo struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	SetupURL    string `json:"setup_url"`
	Notes       string `json:"notes"`
	Required    bool   `json:"required"`
}

type updateCheckResult struct {
	UpdateAvailable bool   `json:"update_available"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	DownloadURL     string `json:"download_url"`
	SetupURL        string `json:"setup_url"`
	Notes           string `json:"notes"`
	Required        bool   `json:"required"`
}

// GetAppVersionJSON returns the running app version for the UI.
func (a *App) GetAppVersionJSON() string {
	return fmt.Sprintf(`{"version":%q}`, GetAppVersion())
}

// CheckForUpdateJSON compares the running app with the server release manifest.
func (a *App) CheckForUpdateJSON() string {
	current := GetAppVersion()
	out := updateCheckResult{
		CurrentVersion: current,
		LatestVersion:  current,
		DownloadURL:    "https://mashad.shop/mscale/download.html",
	}

	req, err := http.NewRequest("GET", apiURL("/api/app/latest"), nil)
	if err != nil {
		b, _ := json.Marshal(out)
		return string(b)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		b, _ := json.Marshal(out)
		return string(b)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := json.Marshal(out)
		return string(b)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		b, _ := json.Marshal(out)
		return string(b)
	}

	var rel releaseInfo
	if err := json.Unmarshal(body, &rel); err != nil || strings.TrimSpace(rel.Version) == "" {
		b, _ := json.Marshal(out)
		return string(b)
	}

	out.LatestVersion = strings.TrimSpace(rel.Version)
	out.Notes = strings.TrimSpace(rel.Notes)
	out.Required = rel.Required
	if u := strings.TrimSpace(rel.SetupURL); u != "" {
		out.SetupURL = u
	}
	if u := strings.TrimSpace(rel.DownloadURL); u != "" {
		out.DownloadURL = u
	}
	if out.SetupURL == "" {
		out.SetupURL = out.DownloadURL
	}

	if versionLess(current, out.LatestVersion) {
		out.UpdateAvailable = true
	}

	b, _ := json.Marshal(out)
	return string(b)
}

func versionLess(current, latest string) bool {
	c := parseVersionParts(current)
	l := parseVersionParts(latest)
	n := len(c)
	if len(l) > n {
		n = len(l)
	}
	for i := 0; i < n; i++ {
		cv, lv := 0, 0
		if i < len(c) {
			cv = c[i]
		}
		if i < len(l) {
			lv = l[i]
		}
		if cv < lv {
			return true
		}
		if cv > lv {
			return false
		}
	}
	return false
}

func parseVersionParts(v string) []int {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	if v == "" {
		return []int{0}
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			out = append(out, 0)
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			out = append(out, 0)
			continue
		}
		out = append(out, n)
	}
	return out
}
