package api

import "sync"

// exitClientState tracks hub routing for one exit-via client (keyed by client overlay IP).
type exitClientState struct {
	exitPeer    string
	exitOverlay string
}

var (
	exitRouteMu       sync.Mutex
	activeExitClients = make(map[string]exitClientState)
)

// clearExitClientRouteLocked removes ip policy for one client only; other clients are untouched.
func clearExitClientRouteLocked(clientOverlay string) {
	if clientOverlay == "" {
		return
	}
	if _, ok := activeExitClients[clientOverlay]; ok {
		clearClientExitPolicy(clientOverlay)
		delete(activeExitClients, clientOverlay)
	}
}

func setActiveExitClientLocked(clientOverlay, exitPeer, exitOverlay string) {
	if clientOverlay == "" {
		return
	}
	activeExitClients[clientOverlay] = exitClientState{
		exitPeer:    exitPeer,
		exitOverlay: exitOverlay,
	}
}
