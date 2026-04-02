package websocket

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
	sendBufferSize = 100
)

type EventType string

const (
	EventPing   EventType = "ping"
	EventPong   EventType = "pong"
	EventNotify EventType = "notify"
	EventKick   EventType = "kick"
	EventJoin   EventType = "join"
	EventQuit   EventType = "quit"
)

type WResponse struct {
	Event     EventType   `json:"event"`
	Data      interface{} `json:"data,omitempty"`
	Code      int         `json:"code"`
	ErrorMsg  string      `json:"errorMsg,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

type WRequest struct {
	Event EventType   `json:"event"`
	Data  interface{} `json:"data,omitempty"`
}

type Client struct {
	ID            string
	UserID        string
	Conn          *websocket.Conn
	Send          chan *WResponse
	mu            sync.Mutex
	closed        bool
	HeartbeatTime time.Time
	IP            string
	UserAgent     string
}

func NewClient(id, userID string, conn *websocket.Conn, ip, userAgent string) *Client {
	return &Client{
		ID:            id,
		UserID:        userID,
		Conn:          conn,
		Send:          make(chan *WResponse, sendBufferSize),
		HeartbeatTime: time.Now(),
		IP:            ip,
		UserAgent:     userAgent,
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			c.mu.Lock()
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				c.mu.Unlock()
				return
			}
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.Conn.WriteJSON(msg)
			c.mu.Unlock()
			if err != nil {
				return
			}
		case <-ticker.C:
			c.mu.Lock()
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.Conn.WriteJSON(&WResponse{
				Event:     EventPing,
				Timestamp: time.Now().Unix(),
			})
			c.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

func (c *Client) ReadPump(onMessage func(*Client, *WRequest)) {
	defer c.Close()
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		c.HeartbeatTime = time.Now()
		return nil
	})

	for {
		var req WRequest
		err := c.Conn.ReadJSON(&req)
		if err != nil {
			break
		}
		if req.Event == EventPong {
			c.HeartbeatTime = time.Now()
			continue
		}
		if onMessage != nil {
			onMessage(c, &req)
		}
	}
}

func (c *Client) SendMsg(msg *WResponse) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	select {
	case c.Send <- msg:
		return true
	default:
		return false
	}
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.Send)
	c.Conn.Close()
}
