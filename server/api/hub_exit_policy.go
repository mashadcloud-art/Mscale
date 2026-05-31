package api

import (
	"fmt"
	"os/exec"
	"strings"
)

const exitPolicyTable = "100"

// ensureClientExitPolicy sends this client's internet-bound forwarded traffic out wg0 (to exit peer).
func ensureClientExitPolicy(clientOverlay string) error {
	if clientOverlay == "" {
		return fmt.Errorf("empty client overlay")
	}
	clearClientExitPolicy(clientOverlay)

	if _, err := runSudo("ip", "rule", "add", "pref", "100",
		"from", clientOverlay+"/32", "lookup", exitPolicyTable); err != nil {
		return fmt.Errorf("ip rule: %w", err)
	}
	for _, prefix := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if _, err := runSudo("ip", "route", "replace", prefix,
			"dev", "wg0", "table", exitPolicyTable); err != nil {
			return fmt.Errorf("ip route table %s: %w", exitPolicyTable, err)
		}
	}
	return verifyHubExitRoute(clientOverlay)
}

func verifyHubExitRoute(clientOverlay string) error {
	out, err := runSudo("ip", "route", "get", "8.8.8.8", "from", clientOverlay, "iif", "wg0")
	if err != nil {
		return fmt.Errorf("route verify: %w", err)
	}
	if !strings.Contains(out, "dev wg0") {
		return fmt.Errorf("route verify: expected dev wg0, got: %s", out)
	}
	if !strings.Contains(out, "table "+exitPolicyTable) {
		return fmt.Errorf("route verify: expected table %s, got: %s", exitPolicyTable, out)
	}
	return nil
}

func clearClientExitPolicy(clientOverlay string) {
	if clientOverlay == "" {
		return
	}
	for i := 0; i < 32; i++ {
		out, _ := exec.Command("sudo", "ip", "rule", "del", "from", clientOverlay+"/32").CombinedOutput()
		if strings.Contains(string(out), "No such file") || strings.Contains(string(out), "Cannot find") {
			break
		}
	}
}

func ensureHubExitNAT() error {
	check := exec.Command("sudo", "iptables", "-t", "nat", "-C", "POSTROUTING", "-s", "100.64.0.0/10", "!", "-d", "100.64.0.0/10", "-j", "MASQUERADE")
	if check.Run() == nil {
		return nil
	}
	if _, err := runSudo("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", "100.64.0.0/10", "!", "-d", "100.64.0.0/10", "-j", "MASQUERADE"); err != nil {
		return err
	}
	return nil
}
