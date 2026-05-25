package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// ─── Config ──────────────────────────────────────────────────────────────────

const (
	serverAddr  = "129.151.146.44"
	registerURL = "http://" + serverAddr + ":8081/register"
	statusURL   = "http://" + serverAddr + ":8081/status"
	wgEndpoint  = serverAddr + ":51820"

	// overlayNet controls what traffic goes through the tunnel:
	// "0.0.0.0/0"       = ALL internet (exit node mode — your IP becomes the server's IP)
	// "100.64.0.0/10"   = overlay only (mesh VPN mode — only VPN traffic goes through)
	overlayNet  = "0.0.0.0/0"
	overlayMask = "0.0.0.0"

	tunMTU    = 1420
	keepalive = 25

	healthCheckInterval = 30 * time.Second
	maxFailedChecks     = 3
)

// ─── Types ───────────────────────────────────────────────────────────────────

type Peer struct {
	PublicKey string `json:"public_key"`
	IP        string `json:"ip"`
	ServerKey string `json:"server_key"`
}

type TunnelState struct {
	wgEngine    *device.Device
	tunDevice   tun.Device
	adapterName string
	assignedIP  string
}

// ─── HTTP Client ──────────────────────────────────────────────────────────────

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

func authToken() string {
	t := os.Getenv("MSCALE_TOKEN")
	if t == "" {
		fmt.Println("[WARN] MSCALE_TOKEN not set, using default token")
		return "my-secret-token-123"
	}
	return t
}

// ─── Registration ─────────────────────────────────────────────────────────────

func register(publicKey wgtypes.Key) (*Peer, error) {
	payload, err := json.Marshal(Peer{PublicKey: hex.EncodeToString(publicKey[:])})
	if err != nil {
		return nil, fmt.Errorf("marshal failed: %w", err)
	}

	req, err := http.NewRequest("POST", registerURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	req.Header.Set("Authorization", authToken())
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var node Peer
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}
	if node.IP == "" || node.ServerKey == "" {
		return nil, fmt.Errorf("server returned incomplete data")
	}

	return &node, nil
}

// ─── Tunnel Setup ─────────────────────────────────────────────────────────────

func setupTunnel(privateKey wgtypes.Key, node *Peer) (*TunnelState, error) {
	serverKey, err := wgtypes.ParseKey(strings.TrimSpace(node.ServerKey))
	if err != nil {
		return nil, fmt.Errorf("invalid server key: %w", err)
	}

	// Create TUN adapter
	tunDevice, err := tun.CreateTUN("MScale", tunMTU)
	if err != nil {
		return nil, fmt.Errorf("TUN create failed (run as Administrator): %w", err)
	}

	adapterName, err := tunDevice.Name()
	if err != nil {
		tunDevice.Close()
		return nil, fmt.Errorf("get adapter name failed: %w", err)
	}
	fmt.Println("[INFO] Adapter created:", adapterName)

	// Configure WireGuard engine
	logger := device.NewLogger(device.LogLevelError, "[WG] ")
	wgEngine := device.NewDevice(tunDevice, conn.NewDefaultBind(), logger)

	wgConfig := fmt.Sprintf(
		"private_key=%s\npublic_key=%s\nendpoint=%s\nallowed_ip=%s\npersistent_keepalive_interval=%d\n",
		hex.EncodeToString(privateKey[:]),
		hex.EncodeToString(serverKey[:]),
		wgEndpoint,
		overlayNet,
		keepalive,
	)

	if err := wgEngine.IpcSet(wgConfig); err != nil {
		wgEngine.Close()
		return nil, fmt.Errorf("WireGuard config failed: %w", err)
	}

	if err := wgEngine.Up(); err != nil {
		wgEngine.Close()
		return nil, fmt.Errorf("WireGuard up failed: %w", err)
	}

	// Assign IP to adapter
	out, err := exec.Command("netsh", "interface", "ipv4", "set", "address",
		"name="+adapterName, "static", node.IP, "255.192.0.0").CombinedOutput()
	if err != nil {
		wgEngine.Close()
		return nil, fmt.Errorf("IP assign failed: %s", string(out))
	}
	fmt.Printf("[INFO] IP %s/255.192.0.0 assigned to %s\n", node.IP, adapterName)

	// Add route
	out, err = exec.Command("netsh", "interface", "ipv4", "add", "route",
		overlayNet, "name="+adapterName, "store=active").CombinedOutput()
	if err != nil {
		fmt.Println("[WARN] Route add:", strings.TrimSpace(string(out)))
	} else {
		fmt.Println("[INFO] Route added:", overlayNet, "->", adapterName)
	}

	return &TunnelState{
		wgEngine:    wgEngine,
		tunDevice:   tunDevice,
		adapterName: adapterName,
		assignedIP:  node.IP,
	}, nil
}

func (s *TunnelState) teardown() {
	fmt.Println("[INFO] Tearing down tunnel...")
	s.wgEngine.Down()
	exec.Command("netsh", "interface", "ipv4", "delete", "route",
		overlayNet, "name="+s.adapterName).Run()
	s.tunDevice.Close()
	fmt.Println("[INFO] Tunnel closed")
}

// ─── Health Check ─────────────────────────────────────────────────────────────

func isTunnelAlive(serverTunnelIP string) bool {
	cmd := exec.Command("ping", "-n", "1", "-w", "2000", serverTunnelIP)
	return cmd.Run() == nil
}

// ─── Connect ─────────────────────────────────────────────────────────────────

func connect() (*TunnelState, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("keygen failed: %w", err)
	}

	fmt.Println("[INFO] Registering with MScale Server...")
	node, err := register(privateKey.PublicKey())
	if err != nil {
		return nil, fmt.Errorf("registration failed: %w", err)
	}
	fmt.Println("[INFO] Registered. Assigned IP:", node.IP)

	state, err := setupTunnel(privateKey, node)
	if err != nil {
		return nil, fmt.Errorf("tunnel setup failed: %w", err)
	}

	return state, nil
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	var state *TunnelState
	failedChecks := 0

	// Initial connection with retry
	for {
		var err error
		state, err = connect()
		if err != nil {
			fmt.Println("[ERROR]", err)
			fmt.Println("[INFO] Retrying in 10 seconds...")
			time.Sleep(10 * time.Second)
			continue
		}
		break
	}

	fmt.Println("[OK] Tunnel Active. IP:", state.assignedIP)
	fmt.Println("[OK] Exit node enabled — your IP is now", serverAddr)
	fmt.Println("[INFO] Press Ctrl+C to disconnect")

	// Health check ticker
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			state.teardown()
			fmt.Println("[INFO] Goodbye!")
			return

		case <-ticker.C:
			if isTunnelAlive("100.64.0.1") {
				failedChecks = 0
				fmt.Println("[INFO] Tunnel healthy ✓")
			} else {
				failedChecks++
				fmt.Printf("[WARN] Tunnel check failed (%d/%d)\n", failedChecks, maxFailedChecks)

				if failedChecks >= maxFailedChecks {
					fmt.Println("[WARN] Tunnel down — reconnecting...")
					state.teardown()
					failedChecks = 0

					backoff := 5 * time.Second
					for {
						newState, err := connect()
						if err != nil {
							fmt.Printf("[ERROR] Reconnect failed: %v. Retrying in %s...\n", err, backoff)
							time.Sleep(backoff)
							if backoff < 60*time.Second {
								backoff *= 2
							}
							continue
						}
						state = newState
						fmt.Println("[OK] Reconnected! IP:", state.assignedIP)
						break
					}
				}
			}
		}
	}
}