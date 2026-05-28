// mscale-agent — Linux headless mesh + exit node (for cloud VPS e.g. India pilehead).
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	defaultServer = "129.151.146.44"
	defaultPort   = "8081"
	overlayNet    = "100.64.0.0/10"
)

type client struct {
	http     *http.Client
	base     string
	serverIP string
	device   string
	devID    string
	overlay  string
	adapter  string
	wg       *device.Device
	tun      tun.Device
}

func main() {
	email := flag.String("email", os.Getenv("MSCALE_EMAIL"), "login email")
	password := flag.String("password", os.Getenv("MSCALE_PASSWORD"), "login password")
	deviceName := flag.String("name", os.Getenv("MSCALE_DEVICE_NAME"), "device name")
	country := flag.String("country", envOr("MSCALE_COUNTRY", "IN"), "exit node country code (IN, AE, …)")
	server := flag.String("server", envOr("MSCALE_SERVER", defaultServer), "control server IP")
	exitNode := flag.Bool("exit", true, "register as public exit node after connect")
	flag.Parse()

	if *email == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Usage: mscale-agent -email YOU@example.com -password '...' [-name india-exit] [-country IN]")
		fmt.Fprintln(os.Stderr, "Or set MSCALE_EMAIL and MSCALE_PASSWORD.")
		os.Exit(1)
	}
	if *deviceName == "" {
		h, _ := os.Hostname()
		*deviceName = strings.TrimSpace(h)
		if *deviceName == "" {
			*deviceName = "mscale-linux"
		}
	}
	*country = strings.ToUpper(strings.TrimSpace(*country))

	jar, _ := cookiejar.New(nil)
	c := &client{
		http:     &http.Client{Timeout: 20 * time.Second, Jar: jar},
		base:     fmt.Sprintf("http://%s:%s", *server, defaultPort),
		serverIP: *server,
		device:   *deviceName,
	}

	if err := c.login(*email, *password); err != nil {
		fatal(err)
	}
	logf("logged in")

	if err := c.connectMesh(); err != nil {
		fatal(err)
	}
	logf("mesh connected overlay=%s adapter=%s", c.overlay, c.adapter)

	if *exitNode {
		if err := c.enableExit(*country); err != nil {
			fatal(err)
		}
		if err := enableLinuxNAT(c.adapter); err != nil {
			logf("WARN: NAT setup: %v", err)
		}
		logf("exit node enabled country=%s", *country)
	}

	go c.heartbeatLoop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logf("shutting down...")
	c.teardown()
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", err)
	os.Exit(1)
}

func logf(format string, args ...any) {
	fmt.Printf("[mscale-agent] "+format+"\n", args...)
}

func (c *client) api(path string, body []byte) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewBuffer(body)
	}
	req, err := http.NewRequest(http.MethodPost, c.base+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	return c.http.Do(req)
}

func (c *client) get(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	return c.http.Do(req)
}

func (c *client) login(email, password string) error {
	payload, _ := json.Marshal(map[string]string{"email": strings.TrimSpace(email), "password": password})
	resp, err := c.api("/api/auth/login", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return apiErr(resp)
	}
	resp, err = c.get("/api/me")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("session check failed")
	}
	return nil
}

func (c *client) connectMesh() error {
	priv, err := loadOrCreatePrivateKey()
	if err != nil {
		return err
	}
	pub := priv.PublicKey()
	pubHex := hex.EncodeToString(pub[:])

	devID, err := c.registerDevice(pubHex)
	if err != nil {
		return err
	}
	c.devID = devID
	if err := c.updateKey(devID, pubHex); err != nil {
		return err
	}
	enroll, err := c.enroll(devID)
	if err != nil {
		return err
	}
	c.overlay = enroll.OverlayIP

	tunDev, err := tun.CreateTUN("mscale", 1420)
	if err != nil {
		return fmt.Errorf("TUN: %w (need root/CAP_NET_ADMIN)", err)
	}
	c.tun = tunDev
	adapter, err := tunDev.Name()
	if err != nil {
		tunDev.Close()
		return err
	}
	c.adapter = adapter

	logger := device.NewLogger(device.LogLevelError, "[wg] ")
	wgDev := device.NewDevice(tunDev, conn.NewDefaultBind(), logger)
	c.wg = wgDev

	serverKey, err := wgtypes.ParseKey(strings.TrimSpace(enroll.ServerKey))
	if err != nil {
		return fmt.Errorf("server key: %w", err)
	}

	cfg := fmt.Sprintf(
		"private_key=%s\npublic_key=%s\nendpoint=%s:51820\nallowed_ip=%s\npersistent_keepalive_interval=25\n",
		hex.EncodeToString(priv[:]),
		hex.EncodeToString(serverKey[:]),
		c.serverIP,
		overlayNet,
	)
	if err := wgDev.IpcSet(cfg); err != nil {
		return err
	}
	if err := wgDev.Up(); err != nil {
		return err
	}

	if err := configureLinux(adapter, enroll.OverlayIP); err != nil {
		return err
	}
	c.sendHeartbeat("online")
	return nil
}

func configureLinux(adapter, ip string) error {
	run("ip", "link", "set", "dev", adapter, "up")
	if out, err := runOut("ip", "addr", "replace", ip+"/10", "dev", adapter); err != nil {
		return fmt.Errorf("ip addr: %s %w", out, err)
	}
	_ = run("ip", "route", "replace", overlayNet, "dev", adapter)
	return nil
}

func loadOrCreatePrivateKey() (wgtypes.Key, error) {
	path := "/var/lib/mscale/wg_private.key"
	if b, err := os.ReadFile(path); err == nil {
		raw, err := hex.DecodeString(strings.TrimSpace(string(b)))
		if err == nil && len(raw) == 32 {
			var k wgtypes.Key
			copy(k[:], raw)
			return k, nil
		}
	}
	priv, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return wgtypes.Key{}, err
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, []byte(hex.EncodeToString(priv[:])), 0o600)
	return priv, nil
}

func enableLinuxNAT(adapter string) error {
	_ = run("sysctl", "-w", "net.ipv4.ip_forward=1")
	iptEnsure([]string{"-I", "INPUT", "-i", adapter, "-j", "ACCEPT"})
	out, err := exec.Command("ip", "-4", "route", "show", "default").Output()
	if err != nil {
		return err
	}
	fields := strings.Fields(string(out))
	if len(fields) < 5 {
		return fmt.Errorf("no default route")
	}
	wan := fields[4]

	iptEnsure([]string{"-A", "FORWARD", "-i", adapter, "-o", wan, "-j", "ACCEPT"})
	iptEnsure([]string{"-A", "FORWARD", "-i", wan, "-o", adapter, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"})
	iptEnsure([]string{"-t", "nat", "-A", "POSTROUTING", "-s", "100.64.0.0/10", "-o", wan, "-j", "MASQUERADE"})

	logf("NAT/forward enabled: %s -> %s", adapter, wan)
	return nil
}

func iptEnsure(rule []string) {
	args := append([]string{"iptables"}, rule...)
	_ = exec.Command(args[0], args[1:]...).Run()
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func runOut(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	b, err := cmd.CombinedOutput()
	return string(b), err
}

func (c *client) registerDevice(pubHex string) (string, error) {
	payload, _ := json.Marshal(map[string]string{
		"device_name": c.device,
		"platform":    "linux",
		"device_type": "desktop",
		"public_key":  pubHex,
		"tunnel_mode": "mesh",
	})
	resp, err := c.api("/api/devices/register", payload)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", apiErr(resp)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.ID == "" {
		return "", fmt.Errorf("invalid register response")
	}
	_ = os.MkdirAll(filepath.Dir(statePath()), 0o700)
	_ = os.WriteFile(statePath(), []byte(out.ID), 0o600)
	return out.ID, nil
}

func statePath() string {
	return "/var/lib/mscale/device_id"
}

func (c *client) updateKey(deviceID, pubHex string) error {
	payload, _ := json.Marshal(map[string]string{"device_id": deviceID, "public_key": pubHex})
	resp, err := c.api("/api/devices/update-key", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return apiErr(resp)
	}
	return nil
}

func (c *client) enroll(deviceID string) (*enrollResp, error) {
	payload, _ := json.Marshal(map[string]string{"device_id": deviceID})
	resp, err := c.api("/api/devices/enroll", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiErr(resp)
	}
	var out enrollResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

type enrollResp struct {
	OverlayIP string `json:"overlay_ip"`
	ServerKey string `json:"server_key"`
}

func (c *client) enableExit(country string) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"device_id":    c.devID,
		"label":        c.device + " (India VPS)",
		"country_code": country,
		"is_private":   false,
	})
	resp, err := c.api("/api/devices/exit-node/enable", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return apiErr(resp)
	}
	return nil
}

func (c *client) sendHeartbeat(status string) {
	if c.devID == "" {
		return
	}
	payload, _ := json.Marshal(map[string]string{"status": status, "peer_id": c.devID})
	resp, err := c.api("/status/update", payload)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func (c *client) heartbeatLoop() {
	t := time.NewTicker(25 * time.Second)
	defer t.Stop()
	for range t.C {
		c.sendHeartbeat("online")
	}
}

func (c *client) teardown() {
	c.sendHeartbeat("offline")
	if c.wg != nil {
		c.wg.Down()
		c.wg.Close()
	}
	if c.tun != nil {
		c.tun.Close()
	}
}

func apiErr(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}
