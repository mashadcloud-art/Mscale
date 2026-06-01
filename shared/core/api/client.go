package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
)

const (
	AppVersion    = "1.0.16"
	sessionCookie = "mscale_session"
)

// Client wraps all API operations to the Mscale backend.
type Client struct {
	Token           string
	APIBase         string
	NeedsProtection bool
	ProtectFD       func(fd int) bool
}

// NewClient initializes an API client.
func NewClient(token, apiBase string, needsProtection bool, protectFD func(fd int) bool) *Client {
	if apiBase == "" {
		apiBase = "https://mashad.shop/mscale"
	}
	return &Client{
		Token:           token,
		APIBase:         apiBase,
		NeedsProtection: needsProtection,
		ProtectFD:       protectFD,
	}
}

func (c *Client) Call(method, path string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.APIBase+path, reader)
	if err != nil {
		return nil, err
	}
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: c.Token})
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	httpClient := http.DefaultClient
	if c.NeedsProtection && c.ProtectFD != nil {
		httpClient = c.protectedHTTPClient()
	}
	return httpClient.Do(req)
}

func (c *Client) protectedHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := net.Dialer{}
				conn, err := d.DialContext(ctx, network, addr)
				if err == nil {
					c.protectConn(conn)
				}
				return conn, err
			},
		},
	}
}

func (c *Client) protectConn(conn net.Conn) {
	if c.ProtectFD == nil {
		return
	}
	rc, ok := conn.(syscall.Conn)
	if !ok {
		return
	}
	raw, err := rc.SyscallConn()
	if err != nil {
		return
	}
	_ = raw.Control(func(fd uintptr) {
		c.ProtectFD(int(fd))
	})
}

// ParseAPIError attempts to extract the JSON error message from the response.
func ParseAPIError(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	var out struct {
		Error   string `json:"error"`
		Details string `json:"details"`
	}
	if json.Unmarshal(b, &out) == nil && out.Error != "" {
		if out.Details != "" {
			return out.Error + ": " + out.Details
		}
		return out.Error
	}
	return fmt.Sprintf("HTTP %d", resp.StatusCode)
}
