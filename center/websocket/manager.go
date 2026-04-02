package websocket

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Hub struct {
	clientsMu   sync.RWMutex
	clients     map[string]*Client
	userClients map[string]map[string]*Client

	register   chan *Client
	unregister chan *Client
	broadcast  chan *WResponse
	usercast   chan *userMessage
	stop       chan struct{}

	onMessage func(*Client, *WRequest)
}

type userMessage struct {
	userID string
	msg    *WResponse
}

var hub *Hub
var hubOnce sync.Once

func GetHub() *Hub {
	hubOnce.Do(func() {
		hub = &Hub{
			clients:     make(map[string]*Client),
			userClients: make(map[string]map[string]*Client),
			register:    make(chan *Client, 100),
			unregister:  make(chan *Client, 100),
			broadcast:   make(chan *WResponse, 100),
			usercast:    make(chan *userMessage, 100),
			stop:        make(chan struct{}),
		}
	})
	return hub
}

func (h *Hub) SetOnMessage(fn func(*Client, *WRequest)) {
	h.onMessage = fn
}

func (h *Hub) Run() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case client := <-h.register:
			if client != nil {
				h.registerClient(client)
			}

		case client := <-h.unregister:
			if client != nil {
				h.unregisterClient(client)
			}

		case msg := <-h.broadcast:
			if msg != nil {
				h.broadcastMessage(msg)
			}

		case um := <-h.usercast:
			if um != nil {
				h.sendToUser(um.userID, um.msg)
			}

		case <-ticker.C:
			h.cleanupStaleConnections()

		case <-h.stop:
			return
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	h.clients[client.ID] = client

	if _, ok := h.userClients[client.UserID]; !ok {
		h.userClients[client.UserID] = make(map[string]*Client)
	}
	h.userClients[client.UserID][client.ID] = client
}

func (h *Hub) unregisterClient(client *Client) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	if _, ok := h.clients[client.ID]; ok {
		delete(h.clients, client.ID)
	}

	if userClients, ok := h.userClients[client.UserID]; ok {
		delete(userClients, client.ID)
		if len(userClients) == 0 {
			delete(h.userClients, client.UserID)
		}
	}
}

func (h *Hub) broadcastMessage(msg *WResponse) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	for _, client := range h.clients {
		go client.SendMsg(msg)
	}
}

func (h *Hub) sendToUser(userID string, msg *WResponse) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	if userClients, ok := h.userClients[userID]; ok {
		for _, client := range userClients {
			go client.SendMsg(msg)
		}
	}
}

func (h *Hub) cleanupStaleConnections() {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	timeout := time.Now().Add(-5 * time.Minute)
	for _, client := range h.clients {
		if client.HeartbeatTime.Before(timeout) {
			go client.Close()
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) Broadcast(msg *WResponse) {
	h.broadcast <- msg
}

func (h *Hub) SendToUser(userID string, msg *WResponse) {
	h.usercast <- &userMessage{userID: userID, msg: msg}
}

func (h *Hub) SendToUserDirect(userID string, msg *WResponse) int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	count := 0
	if userClients, ok := h.userClients[userID]; ok {
		for _, client := range userClients {
			if client.SendMsg(msg) {
				count++
			}
		}
	}
	return count
}

func (h *Hub) GetOnlineCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

func (h *Hub) GetOnlineUserCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.userClients)
}

func (h *Hub) IsUserOnline(userID string) bool {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	_, ok := h.userClients[userID]
	return ok
}

func (h *Hub) Stop() {
	close(h.stop)
}

func GenerateClientID() string {
	return uuid.New().String()
}
