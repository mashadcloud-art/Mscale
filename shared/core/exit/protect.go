package exit

// SetSocketProtect registers VpnService.Protect for outbound exit-forward sockets.
func SetSocketProtect(fn func(fd int) bool) {
	socketProtect = fn
}

var socketProtect func(fd int) bool
