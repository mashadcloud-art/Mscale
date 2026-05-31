package mscalecore

import (
	"syscall"
	_ "unsafe"
)

// WireGuard UDP must bypass the Android VPN tunnel in exit-via mode, otherwise
// hub traffic loops through the tunnel and dies after a few seconds.
//
//go:linkname controlFns golang.zx2c4.com/wireguard/conn.controlFns
var controlFns []func(network, address string, c syscall.RawConn) error

var wgProtectFn func(fd int) bool
var wgProtectInstalled bool

func installWireGuardProtect(protect func(fd int) bool) {
	wgProtectFn = protect
	if protect == nil || wgProtectInstalled {
		return
	}
	wgProtectInstalled = true
	controlFns = append(controlFns, func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			if wgProtectFn != nil {
				wgProtectFn(int(fd))
			}
		})
	})
}
