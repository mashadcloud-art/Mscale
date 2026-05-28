//go:build !windows

package main

func ensureWintunDLL() error {
	return nil
}
