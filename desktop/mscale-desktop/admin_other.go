//go:build !windows

package main

func isProcessElevated() bool {
	return true
}

func (a *App) GetSystemStatus() string {
	return `{"elevated":true,"needs_admin":false,"message":"OK"}`
}

func (a *App) RestartAsAdministrator() string {
	return "Success: Not required on this platform."
}
