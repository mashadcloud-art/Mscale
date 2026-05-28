package api

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func runSudo(args ...string) (string, error) {
	cmd := exec.Command("sudo", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(string(out)), nil
}
