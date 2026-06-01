package exit

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/tun"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

var bufferPool = sync.Pool{
	New: func() any { b := make([]byte, 4096); return &b },
}

// ProxyTun creates a TUN device that intercepts internet-bound traffic
// and routes it through a gVisor user-space network stack.
type ProxyTun struct {
	baseTun tun.Device
	ep      *channel.Endpoint
	s       *stack.Stack

	events chan tun.Event
	mtu    int
	closed bool
	mu     sync.Mutex
}

// NewProxyTun initializes the gVisor stack and forwarders
func NewProxyTun(base tun.Device, mtu int) (*ProxyTun, error) {
	s := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
	})

	ep := channel.New(1024, uint32(mtu), "")

	if err := s.CreateNIC(1, ep); err != nil {
		return nil, fmt.Errorf("could not create NIC: %v", err)
	}

	// Route all traffic to this NIC
	s.SetRouteTable([]tcpip.Route{
		{
			Destination: header.IPv4EmptySubnet,
			NIC:         1,
		},
		{
			Destination: header.IPv6EmptySubnet,
			NIC:         1,
		},
	})

	s.SetForwardingDefaultAndAllNICs(ipv4.ProtocolNumber, true)
	s.SetForwardingDefaultAndAllNICs(ipv6.ProtocolNumber, true)

	// TCP Forwarder
	tcpForwarder := tcp.NewForwarder(s, 0, 1024, func(r *tcp.ForwarderRequest) {
		var wq waiter.Queue
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			r.Complete(true)
			return
		}
		r.Complete(false)

		conn := gonet.NewTCPConn(&wq, ep)

		targetIP := r.ID().LocalAddress.String()
		targetPort := r.ID().LocalPort
		target := fmt.Sprintf("%s:%d", targetIP, targetPort)

		go func() {
			defer conn.Close()
			outConn, err := dialProtected("tcp", target, 10*time.Second)
			if err != nil {
				return
			}
			defer outConn.Close()

			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); io.Copy(outConn, conn) }()
			go func() { defer wg.Done(); io.Copy(conn, outConn) }()
			wg.Wait()
		}()
	})
	s.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	// UDP Forwarder
	udpForwarder := udp.NewForwarder(s, func(r *udp.ForwarderRequest) {
		var wq waiter.Queue
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			return
		}

		conn := gonet.NewUDPConn(&wq, ep)

		targetIP := r.ID().LocalAddress.String()
		targetPort := r.ID().LocalPort
		target := fmt.Sprintf("%s:%d", targetIP, targetPort)

		go func() {
			defer conn.Close()
			outConn, err := dialProtected("udp", target, 5*time.Second)
			if err != nil {
				return
			}
			defer outConn.Close()

			go func() {
				buf := make([]byte, 4096)
				for {
					outConn.SetReadDeadline(time.Now().Add(60 * time.Second))
					n, err := outConn.Read(buf)
					if err != nil {
						break
					}
					conn.Write(buf[:n])
				}
			}()

			buf := make([]byte, 4096)
			for {
				n, err := conn.Read(buf)
				if err != nil {
					break
				}
				outConn.Write(buf[:n])
			}
		}()
	})
	s.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)

	pt := &ProxyTun{
		baseTun: base,
		ep:      ep,
		s:       s,
		events:  make(chan tun.Event, 5),
		mtu:     mtu,
	}
	pt.events <- tun.EventUp

	go pt.readFromGvisor()

	return pt, nil
}

func (pt *ProxyTun) readFromGvisor() {
	for {
		pkt := pt.ep.ReadContext(context.Background())
		if pkt == nil {
			return // endpoint closed
		}

		view := pkt.ToView()
		buf := view.AsSlice()

		_, err := pt.baseTun.Write([][]byte{buf}, 0)
		pkt.DecRef()

		if err != nil {
			return
		}
	}
}

func (pt *ProxyTun) File() *os.File {
	return pt.baseTun.File()
}

func (pt *ProxyTun) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	return pt.baseTun.Read(bufs, sizes, offset)
}

func (pt *ProxyTun) Write(bufs [][]byte, offset int) (int, error) {
	for i, buf := range bufs {
		packet := buf[offset:]
		if len(packet) >= 20 {
			version := packet[0] >> 4
			if version == 4 {
				destIP := packet[16:20]
				// 100.64.0.0/10 is mesh traffic
				if destIP[0] == 100 && (destIP[1] >= 64 && destIP[1] <= 127) {
					_, err := pt.baseTun.Write([][]byte{buf}, offset)
					if err != nil {
						return i, err
					}
					continue
				}

				// Internet traffic! Inject into gVisor
				pktBuf := stack.NewPacketBuffer(stack.PacketBufferOptions{
					Payload: buffer.MakeWithData(packet),
				})
				pt.ep.InjectInbound(ipv4.ProtocolNumber, pktBuf)
				continue
			} else if version == 6 {
				pktBuf := stack.NewPacketBuffer(stack.PacketBufferOptions{
					Payload: buffer.MakeWithData(packet),
				})
				pt.ep.InjectInbound(ipv6.ProtocolNumber, pktBuf)
				continue
			}
		}

		// Fallback to native
		_, err := pt.baseTun.Write([][]byte{buf}, offset)
		if err != nil {
			return i, err
		}
	}
	return len(bufs), nil
}

func (pt *ProxyTun) MTU() (int, error) {
	return pt.mtu, nil
}

func (pt *ProxyTun) Name() (string, error) {
	return "proxy-tun", nil
}

func (pt *ProxyTun) Events() <-chan tun.Event {
	return pt.events
}

func (pt *ProxyTun) Close() error {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if pt.closed {
		return nil
	}
	pt.closed = true

	pt.ep.Close()
	pt.s.Close()
	return pt.baseTun.Close()
}

func (pt *ProxyTun) BatchSize() int {
	return pt.baseTun.BatchSize()
}

func dialProtected(network, address string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	if socketProtect != nil {
		protect := socketProtect
		d.Control = func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				protect(int(fd))
			})
		}
	}
	return d.Dial(network, address)
}
