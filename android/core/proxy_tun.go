package mscalecore

import (
	"fmt"
	"os"
	"sync"
	"syscall"

	"github.com/xjasonlyu/tun2socks/v2/engine"
	"golang.zx2c4.com/wireguard/tun"
)

var bufferPool = sync.Pool{
	New: func() any { b := make([]byte, 4096); return &b },
}

type packetData struct {
	data []byte
	n    int
	err  error
}

type ProxyTun struct {
	baseTun      tun.Device
	proxyWriteFD *os.File // We write internet-bound packets here
	proxyReadFD  *os.File // tun2socks reads/writes from here

	packets chan packetData
	events  chan tun.Event
	mtu     int
	closed  bool
	mu      sync.Mutex
}

// NewProxyTun creates a TUN device that splits traffic between Android OS and tun2socks
func NewProxyTun(base tun.Device, mtu int) (*ProxyTun, error) {
	// Create a Unix Datagram socket pair. It behaves identically to a TUN interface.
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return nil, fmt.Errorf("socketpair failed: %w", err)
	}

	// fds[0] goes to tun2socks, fds[1] stays with ProxyTun
	f1 := os.NewFile(uintptr(fds[0]), "proxy-tun2socks")
	f2 := os.NewFile(uintptr(fds[1]), "proxy-wireguard")

	pt := &ProxyTun{
		baseTun:      base,
		proxyWriteFD: f2,
		proxyReadFD:  f1,
		packets:      make(chan packetData, 1024),
		events:       make(chan tun.Event, 5),
		mtu:          mtu,
	}
	pt.events <- tun.EventUp

	// Initialize tun2socks engine
	engine.Insert(&engine.Key{
		Proxy:    "direct://",
		Device:   fmt.Sprintf("fd://%d", f1.Fd()),
		MTU:      mtu,
		LogLevel: "error", // Keep logs clean
	})
	go engine.Start()

	// Start readers
	go pt.readFromBaseTun()
	go pt.readFromProxy()

	return pt, nil
}

func (pt *ProxyTun) readFromBaseTun() {
	for {
		bPtr := bufferPool.Get().(*[]byte)
		buf := *bPtr
		// baseTun expects a 2D slice for batching
		sizes := make([]int, 1)
		bufs := [][]byte{buf}
		n, err := pt.baseTun.Read(bufs, sizes, 0)
		
		if err != nil {
			pt.packets <- packetData{err: err}
			return
		}
		if n > 0 && sizes[0] > 0 {
			pt.packets <- packetData{data: buf, n: sizes[0]}
		} else {
			bufferPool.Put(bPtr)
		}
	}
}

func (pt *ProxyTun) readFromProxy() {
	for {
		bPtr := bufferPool.Get().(*[]byte)
		buf := *bPtr
		n, err := pt.proxyWriteFD.Read(buf)
		if err != nil {
			pt.packets <- packetData{err: err}
			return
		}
		if n > 0 {
			pt.packets <- packetData{data: buf, n: n}
		} else {
			bufferPool.Put(bPtr)
		}
	}
}

func (pt *ProxyTun) File() *os.File {
	// ProxyTun multiplexes, so it doesn't have a single underlying file for outside use
	return pt.baseTun.File()
}

func (pt *ProxyTun) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	if len(bufs) == 0 {
		return 0, nil
	}

	pkt := <-pt.packets
	if pkt.err != nil {
		return 0, pkt.err
	}

	// Copy data to WireGuard's buffer
	copied := copy(bufs[0][offset:], pkt.data[:pkt.n])
	sizes[0] = copied

	// Return buffer to pool
	bPtr := &pkt.data
	bufferPool.Put(bPtr)

	return 1, nil
}

func (pt *ProxyTun) Write(bufs [][]byte, offset int) (int, error) {
	for i, buf := range bufs {
		packet := buf[offset:]
		if len(packet) >= 20 && (packet[0]>>4) == 4 { // IPv4
			destIP := packet[16:20]
			// Check if packet is destined for the mesh overlay (100.64.0.0/10)
			if destIP[0] == 100 && (destIP[1] >= 64 && destIP[1] <= 127) {
				// Local mesh traffic (pinging this phone, etc.) -> Route to native Android TUN
				_, err := pt.baseTun.Write([][]byte{buf}, offset)
				if err != nil {
					return i, err
				}
				continue
			}

			// Internet traffic (e.g. 8.8.8.8) -> Route to tun2socks proxy
			_, err := pt.proxyWriteFD.Write(packet)
			if err != nil {
				return i, err
			}
			continue
		}

		// Non-IPv4 or unknown, route to native
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

	func() {
		defer func() { recover() }()
		engine.Stop()
	}()
	_ = pt.proxyWriteFD.Close()
	_ = pt.proxyReadFD.Close()
	return pt.baseTun.Close()
}

func (pt *ProxyTun) BatchSize() int {
	return 1
}
