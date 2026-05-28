//go:build !windows

package main

func applyWindowsTunnelRoutes(adapterName, overlayNet string) {}

func removeWindowsTunnelRoutes(adapterName, overlayNet string) {}

func defaultRouteViaAdapter(adapterName string) string {
	return ""
}
