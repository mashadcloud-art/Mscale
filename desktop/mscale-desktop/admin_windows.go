//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type systemStatus struct {
	Elevated      bool   `json:"elevated"`
	NeedsAdmin    bool   `json:"needs_admin"`
	Message       string `json:"message"`
	Executable    string `json:"executable"`
}

func isProcessElevated() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()

	var elevation struct {
		TokenIsElevated uint32
	}
	var outLen uint32
	err = windows.GetTokenInformation(
		token,
		windows.TokenElevation,
		(*byte)(unsafe.Pointer(&elevation)),
		uint32(unsafe.Sizeof(elevation)),
		&outLen,
	)
	return err == nil && elevation.TokenIsElevated != 0
}

// GetSystemStatus reports whether the process can create a Wintun adapter (requires elevation on Windows).
func (a *App) GetSystemStatus() string {
	exe, _ := os.Executable()
	st := systemStatus{
		Elevated:   isProcessElevated(),
		Executable: exe,
	}
	if st.Elevated {
		st.Message = "Running with administrator rights — VPN tunnel is allowed."
	} else {
		st.NeedsAdmin = true
		st.Message = "VPN needs administrator rights. Close this window and open the app again from the UAC prompt, or click Restart as Administrator."
	}
	b, _ := json.Marshal(st)
	return string(b)
}

// RestartAsAdministrator relaunches this executable with UAC elevation.
func (a *App) RestartAsAdministrator() string {
	if isProcessElevated() {
		return "Success: Already running as administrator."
	}
	exe, err := os.Executable()
	if err != nil {
		return "Error: " + err.Error()
	}
	cwd, _ := os.Getwd()

	verb, _ := windows.UTF16PtrFromString("runas")
	exePtr, _ := windows.UTF16PtrFromString(exe)
	cwdPtr, _ := windows.UTF16PtrFromString(cwd)
	op, _ := windows.UTF16PtrFromString("")

	shell32 := windows.NewLazyDLL("shell32.dll")
	shellExecuteW := shell32.NewProc("ShellExecuteW")
	ret, _, _ := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(exePtr)),
		uintptr(unsafe.Pointer(op)),
		uintptr(unsafe.Pointer(cwdPtr)),
		1,
	)
	if ret <= 32 {
		return fmt.Sprintf("Error: Could not elevate (code %d). Right-click mscale-desktop.exe → Run as administrator.", ret)
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
	return "Success: Accept the UAC prompt to restart MScale as administrator."
}
