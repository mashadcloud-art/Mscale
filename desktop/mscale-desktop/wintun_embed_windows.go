//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed third_party/wintun/amd64/wintun.dll
var embeddedWintunDLL []byte

// ensureWintunDLL places wintun.dll beside the executable (required by the Wintun driver loader).
func ensureWintunDLL() error {
	if len(embeddedWintunDLL) == 0 {
		return fmt.Errorf("embedded wintun.dll is missing from the build")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	target := filepath.Join(filepath.Dir(exe), "wintun.dll")
	if st, err := os.Stat(target); err == nil && st.Size() > 0 {
		return nil
	}
	if err := os.WriteFile(target, embeddedWintunDLL, 0o644); err != nil {
		return fmt.Errorf("could not write wintun.dll next to %s: %w", exe, err)
	}
	return nil
}
