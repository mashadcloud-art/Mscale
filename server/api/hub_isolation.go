package api

import (
	"database/sql"
	"log"
	"os/exec"
)

// SyncUserIsolationFirewall creates iptables and ipset rules to ensure that
// devices owned by different users cannot communicate with each other over wg0.
func SyncUserIsolationFirewall(db *sql.DB) {
	// 1. Group active overlay IPs by user_id
	rows, err := db.Query("SELECT user_id, overlay_ip FROM devices WHERE overlay_ip IS NOT NULL AND overlay_ip != ''")
	if err != nil {
		log.Printf("ERROR: SyncUserIsolationFirewall db query failed: %v", err)
		return
	}
	// No defer rows.Close() here, we close it early

	userIPs := make(map[string][]string)
	for rows.Next() {
		var userID, ip string
		if err := rows.Scan(&userID, &ip); err == nil {
			userIPs[userID] = append(userIPs[userID], ip)
		}
	}
	rows.Close()

	// 2. Ensure MSCALE_ISOLATION iptables chain exists
	_ = exec.Command("sudo", "iptables", "-N", "MSCALE_ISOLATION").Run()
	_ = exec.Command("sudo", "iptables", "-C", "FORWARD", "-i", "wg0", "-o", "wg0", "-j", "MSCALE_ISOLATION").Run()
	if err := exec.Command("sudo", "iptables", "-C", "FORWARD", "-i", "wg0", "-o", "wg0", "-j", "MSCALE_ISOLATION").Run(); err != nil {
		_ = exec.Command("sudo", "iptables", "-I", "FORWARD", "1", "-i", "wg0", "-o", "wg0", "-j", "MSCALE_ISOLATION").Run()
	}

	// Flush the chain before rebuilding
	_ = exec.Command("sudo", "iptables", "-F", "MSCALE_ISOLATION").Run()

	// 3. Create accept rules for each user's IPs
	for _, ips := range userIPs {
		if len(ips) < 2 {
			continue // No need for isolation rules if the user only has 1 device
		}
		
		// For every pair of IPs owned by the same user, allow forwarding
		for _, srcIP := range ips {
			for _, dstIP := range ips {
				_ = exec.Command("sudo", "iptables", "-A", "MSCALE_ISOLATION", "-s", srcIP, "-d", dstIP, "-j", "ACCEPT").Run()
			}
		}
	}

	// Allow internet-bound traffic (exit node traffic) to bypass isolation.
	// Traffic is only here if WireGuard and ip rules allowed it.
	_ = exec.Command("sudo", "bash", "-c", "iptables -A MSCALE_ISOLATION ! -d 100.64.0.0/10 -j ACCEPT").Run()

	// 4. Drop everything else traveling wg0 to wg0 (cross-user overlay traffic)
	_ = exec.Command("sudo", "iptables", "-A", "MSCALE_ISOLATION", "-j", "DROP").Run()
	
	log.Println("INFO: User Network Isolation Firewall synced.")
}
