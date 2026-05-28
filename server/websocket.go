package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"mscale-server/api"

	"github.com/gorilla/websocket"
)

type wsClient struct {
	conn   *websocket.Conn
	userID string
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	clients map[*websocket.Conn]*wsClient
	mu      sync.Mutex
	db      *sql.DB
}

var hub = &Hub{
	clients: make(map[*websocket.Conn]*wsClient),
}

func (h *Hub) Run() {
	// connections managed in WsHandler goroutines
}

func (h *Hub) addClient(conn *websocket.Conn, userID string) {
	h.mu.Lock()
	h.clients[conn] = &wsClient{conn: conn, userID: userID}
	h.mu.Unlock()
	log.Println("WS: Admin panel connected for user", userID)
}

func (h *Hub) removeClient(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	conn.Close()
	log.Println("WS: Admin panel disconnected")
}

func (h *Hub) devicesPayloadForUser(userID string) ([]byte, error) {
	devices, err := api.ListDesktopDevicesForUser(h.db, userID)
	if err != nil {
		return nil, err
	}
	if devices == nil {
		devices = []api.DeviceListItem{}
	}
	return json.Marshal(devices)
}

func (h *Hub) sendToUser(userID string) {
	data, err := h.devicesPayloadForUser(userID)
	if err != nil {
		log.Println("WS sendToUser error:", err)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for conn, client := range h.clients {
		if client.userID != userID {
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			delete(h.clients, conn)
			conn.Close()
		}
	}
}

func (h *Hub) BroadcastDevices() {
	if h.db == nil {
		return
	}

	h.mu.Lock()
	userIDs := make(map[string]struct{})
	for _, c := range h.clients {
		if c.userID != "" {
			userIDs[c.userID] = struct{}{}
		}
	}
	h.mu.Unlock()

	for userID := range userIDs {
		h.sendToUser(userID)
	}
}

func WsHandler(auth *api.AuthHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := auth.GetSession(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("WS upgrade error:", err)
			return
		}

		hub.addClient(conn, session.UserID)

		if data, err := hub.devicesPayloadForUser(session.UserID); err == nil {
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}

		go func() {
			defer func() {
				hub.removeClient(conn)
			}()
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					break
				}
			}
		}()
	}
}
