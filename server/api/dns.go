package api

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/miekg/dns"
)

var dnsDB *sql.DB

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)

	for _, q := range r.Question {
		if q.Qtype == dns.TypeA {
			name := strings.TrimSuffix(q.Name, ".")
			
			// If it ends with .mscale, extract the short name
			if strings.HasSuffix(name, ".mscale") {
				shortName := strings.TrimSuffix(name, ".mscale")
				
				ip := lookupDeviceIP(shortName)
				if ip != "" {
					rr, err := dns.NewRR(fmt.Sprintf("%s 60 IN A %s", q.Name, ip))
					if err == nil {
						m.Answer = append(m.Answer, rr)
					}
				}
			}
		}
	}

	_ = w.WriteMsg(m)
}

func lookupDeviceIP(shortName string) string {
	if dnsDB == nil {
		return ""
	}

	var overlayIP string
	// Sanitize DB device_name by lowercasing and replacing spaces with hyphens to match the shortName
	err := dnsDB.QueryRow(
		`SELECT overlay_ip FROM devices 
		 WHERE REPLACE(LOWER(device_name), ' ', '-') = ? 
		 AND overlay_ip IS NOT NULL 
		 LIMIT 1`,
		strings.ToLower(shortName),
	).Scan(&overlayIP)

	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("ERROR: DNS DB lookup failed for %s: %v", shortName, err)
		}
		return ""
	}

	return overlayIP
}

func handleDNSForward(w dns.ResponseWriter, r *dns.Msg) {
	c := new(dns.Client)
	in, _, err := c.Exchange(r, "8.8.8.8:53")
	if err == nil {
		_ = w.WriteMsg(in)
		return
	}
	
	m := new(dns.Msg)
	m.SetReply(r)
	m.SetRcode(r, dns.RcodeServerFailure)
	_ = w.WriteMsg(m)
}

func RunDNSServer(db *sql.DB) {
	dnsDB = db

	dns.HandleFunc("mscale.", handleDNSRequest)
	dns.HandleFunc(".", handleDNSForward)
	
	// Listen on the WireGuard interface IP so it's only accessible to mesh peers
	addr := "100.64.0.1:53"
	server := &dns.Server{Addr: addr, Net: "udp"}

	log.Printf("INFO: Mscale DNS Server listening on %s", addr)
	
	err := server.ListenAndServe()
	if err != nil {
		log.Printf("ERROR: Mscale DNS Server failed: %v", err)
	}
}
