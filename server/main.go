package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"mscale-server/api"
	"mscale-server/db"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Peer struct {
	PublicKey string `json:"public_key"`
	PeerID    string `json:"peer_id,omitempty"`
	IP        string `json:"ip"`
	ServerKey string `json:"server_key"`
}

type PeerRecord struct {
	B64Key       string
	PeerID       string
	IP           string
	RegisteredAt time.Time
}

type PeerStatus struct {
	PeerID   string    `json:"peer_id"`
	Status   string    `json:"status"`
	LastSeen time.Time `json:"last_seen"`
}

var (
	peerCount    = 1
	mu           sync.Mutex
	peerRegistry = make(map[string]PeerRecord)
	peerStatuses = make(map[string]PeerStatus)
)

func getServerPublicKey() (string, error) {
	out, err := exec.Command("sudo", "wg", "show", "wg0", "public-key").Output()
	if err != nil {
		return "", fmt.Errorf("failed to read server public key: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func hexToBase64(hexKey string) (string, error) {
	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("invalid hex key: %w", err)
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("key must be exactly 32 bytes, got %d", len(raw))
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func authMiddleware(authHandler *api.AuthHandler) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if _, err := authHandler.GetSession(r); err == nil {
				next(w, r)
				return
			}
			
			authToken := os.Getenv("MSCALE_TOKEN")
			if authToken == "" {
				authToken = "my-secret-token-123"
			}
			if r.Header.Get("Authorization") == authToken {
				next(w, r)
				return
			}
			
			log.Printf("WARN: Unauthorized request from %s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
	}
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var p Peer
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if p.PublicKey == "" {
		http.Error(w, "Missing public_key", http.StatusBadRequest)
		return
	}
	if p.PeerID == "" {
		p.PeerID = "unknown-peer"
	}

	b64Key, err := hexToBase64(p.PublicKey)
	if err != nil {
		log.Printf("ERROR: Invalid public key from %s: %v", r.RemoteAddr, err)
		http.Error(w, "Invalid public key: must be 32-byte hex string", http.StatusBadRequest)
		return
	}

	mu.Lock()
	if existing, ok := peerRegistry[p.PublicKey]; ok {
		if existing.PeerID == "" && p.PeerID != "" {
			existing.PeerID = p.PeerID
			peerRegistry[p.PublicKey] = existing
		}
		mu.Unlock()

		serverPubKey, err := getServerPublicKey()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		log.Printf("INFO: Peer %s... already registered with IP %s", b64Key[:8], existing.IP)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Peer{
			IP:        existing.IP,
			ServerKey: serverPubKey,
			PeerID:    existing.PeerID,
		})
		return
	}

	peerCount++
	if peerCount > 254 {
		mu.Unlock()
		http.Error(w, "IP pool exhausted", http.StatusServiceUnavailable)
		return
	}

	ip := fmt.Sprintf("100.64.0.%d", peerCount)
	peerRegistry[p.PublicKey] = PeerRecord{
		B64Key:       b64Key,
		PeerID:       p.PeerID,
		IP:           ip,
		RegisteredAt: time.Now(),
	}
	mu.Unlock()

	log.Printf("INFO: New registration from %s → assigning IP %s (peer %s...)", r.RemoteAddr, ip, b64Key[:8])

	cmd := exec.Command("sudo", "wg", "set", "wg0", "peer", b64Key, "allowed-ips", ip+"/32")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("ERROR: Failed to add peer: %v | %s", err, string(out))
		mu.Lock()
		delete(peerRegistry, p.PublicKey)
		peerCount--
		mu.Unlock()
		http.Error(w, "Internal Server Error: could not add peer to wg0", http.StatusInternalServerError)
		return
	}

	if out, err := exec.Command("sudo", "wg-quick", "save", "wg0").CombinedOutput(); err != nil {
		log.Printf("WARN: wg-quick save failed: %v | %s", err, string(out))
	}

	serverPubKey, err := getServerPublicKey()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Printf("SUCCESS: Peer %s... registered with IP %s", b64Key[:8], ip)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Peer{
		IP:        ip,
		ServerKey: serverPubKey,
		PeerID:    p.PeerID,
	})
}

func statusUpdateHandler(authHandler *api.AuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			PeerID string `json:"peer_id"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if payload.PeerID == "" {
			http.Error(w, "Missing peer_id", http.StatusBadRequest)
			return
		}
		if payload.Status == "" || strings.ToLower(payload.Status) == "active" {
			payload.Status = "Online"
		}

		_, err := authHandler.DB.Exec("UPDATE devices SET status = ?, last_seen_at = CURRENT_TIMESTAMP WHERE id = ?", payload.Status, payload.PeerID)
		if err != nil {
			log.Printf("ERROR updating DB for heartbeat: %v", err)
		}

		mu.Lock()
		peerStatuses[payload.PeerID] = PeerStatus{
			PeerID:   payload.PeerID,
			Status:   payload.Status,
			LastSeen: time.Now(),
		}
		mu.Unlock()

		log.Printf("HEARTBEAT: peer_id=%s status=%s from %s", payload.PeerID, payload.Status, r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"received": true}`)
	}
}

func peersHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	type PeerInfo struct {
		PeerID       string    `json:"peer_id"`
		IP           string    `json:"ip"`
		RegisteredAt time.Time `json:"registered_at"`
		LastSeen     string    `json:"last_seen"`
		Status       string    `json:"status"`
	}

	result := make(map[string]PeerInfo)
	for hexKey, record := range peerRegistry {
		short := hexKey
		if len(short) > 8 {
			short = short[:8] + "..."
		}

		lastSeen := "never"
		status := "unknown"
		if record.PeerID != "" {
			if ps, ok := peerStatuses[record.PeerID]; ok {
				lastSeen = ps.LastSeen.Format(time.RFC3339)
				status = ps.Status
			}
		}

		result[short] = PeerInfo{
			PeerID:       record.PeerID,
			IP:           record.IP,
			RegisteredAt: record.RegisteredAt,
			LastSeen:     lastSeen,
			Status:       status,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"ok"}`)
}

func serveDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/dashboard" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "dashboard.html")
}

func main() {
	sqlDB, err := db.Open()
	if err != nil {
		log.Fatalf("CRITICAL: failed to open database: %v", err)
	}
	defer sqlDB.Close()

	if err := db.Migrate(sqlDB); err != nil {
		log.Fatalf("CRITICAL: failed to run migrations: %v", err)
	}

	log.Println("INFO: Database ready")

	serverKey, err := getServerPublicKey()
	if err != nil {
		log.Fatalf("CRITICAL: wg0 not running.\nStart with:\n  sudo wg-quick up wg0\n\nError: %v", err)
	}
	log.Printf("INFO: wg0 is up. Server public key: %s", serverKey)

	port := os.Getenv("MSCALE_PORT")
	if port == "" {
		port = "8081"
	}

	authHandler := &api.AuthHandler{DB: sqlDB}

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveDashboard)
	mux.HandleFunc("/dashboard", serveDashboard)
	mux.HandleFunc("/register", authMiddleware(authHandler)(registerHandler))
	mux.HandleFunc("/status/update", authMiddleware(authHandler)(statusUpdateHandler(authHandler)))
	mux.HandleFunc("/peers", authMiddleware(authHandler)(peersHandler))
	mux.HandleFunc("/health", healthHandler)

	mux.HandleFunc("/api/auth/register", authHandler.Register)
	mux.HandleFunc("/api/auth/login", authHandler.Login)
	mux.HandleFunc("/api/auth/logout", authHandler.Logout)
	mux.HandleFunc("/api/me", authHandler.Me)
	mux.HandleFunc("/api/me/device", authHandler.MeDevice)

	mux.HandleFunc("/api/devices/register", authHandler.RegisterDevice)
	mux.HandleFunc("/api/devices", authHandler.ListDevices)
	mux.HandleFunc("/api/devices/enroll", authHandler.EnrollDevice)
	mux.HandleFunc("/api/devices/update-key", authHandler.UpdateDevicePublicKey)
	mux.HandleFunc("/api/devices/update-meta", authHandler.UpdateDeviceMeta)

	log.Printf("INFO: MScale server listening on :%s", port)
	log.Printf("INFO: Routes: POST /register | POST /status/update | GET /peers | GET /health | POST /api/auth/register | POST /api/auth/login | POST /api/auth/logout | GET /api/me | GET /api/me/device | POST /api/devices/register | GET /api/devices | POST /api/devices/enroll | POST /api/devices/update-key | POST /api/devices/update-meta")
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

