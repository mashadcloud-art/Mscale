package api

import (
	"fmt"
	"os/exec"
	"strings"
)

const exitPolicyTable = "100"

// ensureClientExitPolicy routes this client's forwarded internet traffic to the mobile exit peer on wg0.
func ensureClientExitPolicy(clientOverlay, exitOverlay string) error {
	if clientOverlay == "" {
		return fmt.Errorf("empty client overlay")
	}
	if exitOverlay == "" {
		return fmt.Errorf("empty exit overlay")
	}
	clearClientExitPolicy(clientOverlay)

	if _, err := runSudo("ip", "rule", "add", "pref", "100",
		"from", clientOverlay+"/32", "lookup", exitPolicyTable); err != nil {
		return fmt.Errorf("ip rule: %w", err)
	}
	for _, prefix := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		_, err := runSudo("ip", "route", "replace", prefix,
			"via", exitOverlay, "dev", "wg0", "table", exitPolicyTable)
		if err != nil {
			// Some kernels reject via on wg; fall back to dev-only (WireGuard picks exit peer by AllowedIPs).
			if _, err2 := runSudo("ip", "route", "replace", prefix,
				"dev", "wg0", "table", exitPolicyTable); err2 != nil {
				return fmt.Errorf("ip route table %s: %w", exitPolicyTable, err)
			}
		}
	}
	return verifyHubExitRoute(clientOverlay, exitOverlay)
}

func verifyHubExitRoute(clientOverlay, exitOverlay string) error {
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

// clearHubOverlayNAT removes the broad mesh SNAT rule that sends traffic out Oracle's public IP
// instead of forwarding to a phone exit node.
func clearHubOverlayNAT() {
	for i := 0; i < 8; i++ {
		out, _ := exec.Command("sudo", "iptables", "-t", "nat", "-D", "POSTROUTING",
			"-s", "100.64.0.0/10", "!", "-d", "100.64.0.0/10", "-j", "MASQUERADE").CombinedOutput()
		msg := string(out)
		if strings.Contains(msg, "Bad rule") || strings.Contains(msg, "does a matching rule exist") {
			break
		}
	}
}

// ensureMobileExitPath prepares forwarding for phone/tablet exit nodes (no hub SNAT).
func ensureMobileExitPath() error {
	clearHubOverlayNAT()
	return ensureHubExitForwarding()
}

// ensureHubSelfExitNAT SNAT only when the hub itself is the exit (not for mobile exit forwarding).
func ensureHubSelfExitNAT() error {
	clearHubOverlayNAT()
	check := exec.Command("sudo", "iptables", "-t", "nat", "-C", "POSTROUTING",
		"-s", "100.64.0.1/32", "!", "-d", "100.64.0.0/10", "-j", "MASQUERADE")
	if check.Run() == nil {
		return ensureHubExitForwarding()
	}
	if _, err := runSudo("iptables", "-t", "nat", "-A", "POSTROUTING",
		"-s", "100.64.0.1/32", "!", "-d", "100.64.0.0/10", "-j", "MASQUERADE"); err != nil {
		return err
	}
	return ensureHubExitForwarding()
}

// ensureHubExitNAT is deprecated for mobile exit; kept as alias for hub-self mode only.
func ensureHubExitNAT() error {
	return ensureHubSelfExitNAT()
}
