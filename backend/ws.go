package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // local only — no origin restrictions needed
	},
}

type WSHub struct {
	clients map[*websocket.Conn]bool
	mu      sync.Mutex
	msgCh   chan []byte
}

func newWSHub() *WSHub {
	return &WSHub{
		clients: make(map[*websocket.Conn]bool),
		msgCh:   make(chan []byte, 64),
	}
}

func (h *WSHub) run() {
	for msg := range h.msgCh {
		h.mu.Lock()
		for conn := range h.clients {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				conn.Close()
				delete(h.clients, conn)
			}
		}
		h.mu.Unlock()
	}
}

func (h *WSHub) broadcast(data any) {
	msg, err := json.Marshal(data)
	if err != nil {
		return
	}
	select {
	case h.msgCh <- msg:
	default:
		// drop if channel full (client too slow)
	}
}

func (h *WSHub) register(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()
}

func (h *WSHub) unregister(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}

func (pm *ProcessManager) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WS upgrade error:", err)
		return
	}

	pm.hub.register(conn)
	defer func() {
		pm.hub.unregister(conn)
		conn.Close()
	}()

	// Keep alive — read loop discards client messages (ping/pong handled automatically)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
