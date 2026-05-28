package api

import (
	"fmt"
	"os/exec"
	"strings"
)

const exitPeerInternet = "0.0.0.0/1,128.0.0.0/1"

// hubAllowedIPs builds peer AllowedIPs for the UAE hub.
// - Client/desktop peers: overlay /32 only (never full or split internet on hub).
// - India exit peer only: overlay /32 + exitPeerInternet (via syncExitNodePeer).
// Never 0.0.0.0/0 — hijacks the hub default route and breaks SSH (ops/recover-ph-ssh.sh).
func hubAllowedIPs(overlayIP, extraAllowed string) (string, error) {
	extraAllowed = strings.TrimSpace(extraAllowed)
	if strings.Contains(extraAllowed, "0.0.0.0/0") {
		return "", fmt.Errorf("refusing 0.0.0.0/0 on hub peer (breaks SSH)")
	}
	if extraAllowed != "" && extraAllowed != exitPeerInternet {
		return "", fmt.Errorf("refusing non-exit allowed-ips on hub peer: clients get /32 only; exit node gets %s", exitPeerInternet)
	}
	allowed := overlayIP + "/32"
	if extraAllowed != "" {
		allowed = allowed + "," + extraAllowed
	}
	return allowed, nil
}

// syncWGPeer removes stale peers for this overlay, prunes zombies, then configures wg0.
// Does not run wg-quick save — saving bad AllowedIPs to wg0.conf has locked us out of ph before.
func syncWGPeer(overlayIP, pubKeyHex, extraAllowed string) error {
	b64, err := normalizeWGPublicKey(pubKeyHex)
	if err != nil {
		return err
	}
	allowed, err := hubAllowedIPs(overlayIP, extraAllowed)
	if err != nil {
		return err
	}
	pruneWGZombiePeers()
	removeWGPeersForOverlay(overlayIP, b64)

	args := []string{"wg", "set", "wg0", "peer", b64, "allowed-ips", allowed}
	if strings.Contains(extraAllowed, "/1") {
		args = append(args, "persistent-keepalive", "25")
	}
	if out, err := exec.Command("sudo", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// syncExitNodePeer applies Bug-1 fix: India (exit) peer gets /32 + split internet routes.
func syncExitNodePeer(overlayIP, pubKeyHex string) error {
	return syncWGPeer(overlayIP, pubKeyHex, exitPeerInternet)
}

// removeWGPeersForOverlay removes other hub peers still claiming the same overlay /32.
func removeWGPeersForOverlay(overlayIP, keepB64 string) {
	out, err := exec.Command("sudo", "wg", "show", "wg0", "dump").Output()
	if err != nil {
		return
	}
	needle := overlayIP + "/32"
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		pub := fields[0]
		allowed := fields[4]
		if pub == keepB64 {
			continue
		}
		if strings.Contains(allowed, needle) {
			_ = exec.Command("sudo", "wg", "set", "wg0", "peer", pub, "remove").Run()
		}
	}
}

// pruneWGZombiePeers removes Bug-3 peers: allowed-ips (none) from old reconnects.
func pruneWGZombiePeers() {
	out, err := exec.Command("sudo", "wg", "show", "wg0", "dump").Output()
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		pub := fields[0]
		allowed := strings.TrimSpace(fields[4])
		if allowed == "(none)" || allowed == "" {
			_ = exec.Command("sudo", "wg", "set", "wg0", "peer", pub, "remove").Run()
		}
	}
}

// removeWGPeerByPublicKey removes one peer by hex or base64 pubkey (before re-register).
func removeWGPeerByPublicKey(pubKey string) {
	b64, err := normalizeWGPublicKey(pubKey)
	if err != nil {
		return
	}
	_ = exec.Command("sudo", "wg", "set", "wg0", "peer", b64, "remove").Run()
}
