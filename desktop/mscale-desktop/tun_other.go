//go:build !windows

package main

import (
	"golang.zx2c4.com/wireguard/tun"
)

func createMScaleTUN() (tun.Device, error) {
	return tun.CreateTUN("MScale", 1420)
}
