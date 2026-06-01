package wg

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"syscall"

	"golang.zx2c4.com/wireguard/conn"
)

const hubServerIP = "129.151.146.44"

// protectedBind keeps WireGuard UDP off the full-tunnel VPN (prevents hub routing loop).
type protectedBind struct {
	mu      sync.Mutex
	v4      *net.UDPConn
	protect func(fd int) bool
}

func NewProtectedBind(protect func(fd int) bool) conn.Bind {
	if protect == nil {
		return conn.NewDefaultBind()
	}
	return &protectedBind{protect: protect}
}

func (b *protectedBind) Open(uport uint16) ([]conn.ReceiveFunc, uint16, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.v4 != nil {
		return nil, 0, conn.ErrBindAlreadyOpen
	}

	port := int(uport)
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				if b.protect != nil {
					b.protect(int(fd))
				}
			})
		},
	}
	pc, err := lc.ListenPacket(context.Background(), "udp4", ":"+strconv.Itoa(port))
	if err != nil {
		return nil, 0, err
	}
	b.v4 = pc.(*net.UDPConn)
	uaddr, err := net.ResolveUDPAddr("udp4", b.v4.LocalAddr().String())
	if err != nil {
		b.v4.Close()
		b.v4 = nil
		return nil, 0, err
	}
	return []conn.ReceiveFunc{b.receive}, uint16(uaddr.Port), nil
}

func (b *protectedBind) receive(bufs [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
	b.mu.Lock()
	c := b.v4
	b.mu.Unlock()
	if c == nil {
		return 0, net.ErrClosed
	}
	n, addr, err := c.ReadFromUDP(bufs[0])
	if err != nil {
		return 0, err
	}
	sizes[0] = n
	eps[0] = &conn.StdNetEndpoint{AddrPort: addr.AddrPort()}
	return 1, nil
}

func (b *protectedBind) Send(bufs [][]byte, ep conn.Endpoint) error {
	b.mu.Lock()
	c := b.v4
	b.mu.Unlock()
	if c == nil {
		return syscall.EAFNOSUPPORT
	}
	sep, ok := ep.(*conn.StdNetEndpoint)
	if !ok {
		return conn.ErrWrongEndpointType
	}
	ua := net.UDPAddrFromAddrPort(sep.AddrPort)
	for _, buf := range bufs {
		if _, err := c.WriteToUDP(buf, ua); err != nil {
			return err
		}
	}
	return nil
}

func (b *protectedBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	ap, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}
	return &conn.StdNetEndpoint{AddrPort: ap}, nil
}

func (b *protectedBind) SetMark(_ uint32) error { return nil }

func (b *protectedBind) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.v4 == nil {
		return nil
	}
	err := b.v4.Close()
	b.v4 = nil
	return err
}

func (b *protectedBind) BatchSize() int { return 1 }
