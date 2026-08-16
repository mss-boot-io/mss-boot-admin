package websocket

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestClient_SendMsg(t *testing.T) {
	sendChan := make(chan *WResponse, 1)
	client := &Client{
		ID:     "test-client",
		UserID: "test-user",
		Send:   sendChan,
	}

	msg := &WResponse{
		Event:     EventNotify,
		Code:      200,
		Data:      "test message",
		Timestamp: time.Now().Unix(),
	}

	result := client.SendMsg(msg)
	assert.True(t, result)

	received := <-sendChan
	assert.Equal(t, EventNotify, received.Event)
	assert.Equal(t, 200, received.Code)
}

func TestClient_SendMsg_Closed(t *testing.T) {
	sendChan := make(chan *WResponse, 1)
	client := &Client{
		ID:     "test-client",
		UserID: "test-user",
		Send:   sendChan,
		closed: true,
	}

	msg := &WResponse{
		Event:     EventNotify,
		Code:      200,
		Timestamp: time.Now().Unix(),
	}

	result := client.SendMsg(msg)
	assert.False(t, result)
}

func TestClient_SendMsg_BufferFull(t *testing.T) {
	sendChan := make(chan *WResponse, 1)
	client := &Client{
		ID:     "test-client",
		UserID: "test-user",
		Send:   sendChan,
	}

	sendChan <- &WResponse{Event: EventNotify}

	msg := &WResponse{
		Event:     EventNotify,
		Code:      200,
		Timestamp: time.Now().Unix(),
	}

	result := client.SendMsg(msg)
	assert.False(t, result)
}

func TestHub_RegisterUnregister(t *testing.T) {
	hub := &Hub{
		clients:     make(map[string]*Client),
		userClients: make(map[string]map[string]*Client),
		register:    make(chan *Client, 10),
		unregister:  make(chan *Client, 10),
		broadcast:   make(chan *WResponse, 10),
		usercast:    make(chan *userMessage, 10),
		stop:        make(chan struct{}),
	}

	go hub.Run()
	defer hub.Stop()

	client := &Client{
		ID:     "client-1",
		UserID: "user-1",
		Send:   make(chan *WResponse, 1),
	}

	hub.Register(client)
	time.Sleep(100 * time.Millisecond)

	assert.True(t, hub.IsUserOnline("user-1"))
	assert.Equal(t, 1, hub.GetOnlineCount())
	assert.Equal(t, 1, hub.GetOnlineUserCount())

	hub.Unregister(client)
	time.Sleep(100 * time.Millisecond)

	assert.False(t, hub.IsUserOnline("user-1"))
	assert.Equal(t, 0, hub.GetOnlineCount())
	assert.Equal(t, 0, hub.GetOnlineUserCount())
}

func TestHub_SendToUser(t *testing.T) {
	hub := &Hub{
		clients:     make(map[string]*Client),
		userClients: make(map[string]map[string]*Client),
		register:    make(chan *Client, 10),
		unregister:  make(chan *Client, 10),
		broadcast:   make(chan *WResponse, 10),
		usercast:    make(chan *userMessage, 10),
		stop:        make(chan struct{}),
	}

	go hub.Run()
	defer hub.Stop()

	client := &Client{
		ID:     "client-1",
		UserID: "user-1",
		Send:   make(chan *WResponse, 10),
	}

	hub.Register(client)
	time.Sleep(100 * time.Millisecond)

	msg := &WResponse{
		Event:     EventNotify,
		Code:      200,
		Data:      "test notification",
		Timestamp: time.Now().Unix(),
	}

	count := hub.SendToUserDirect("user-1", msg)
	assert.Equal(t, 1, count)

	select {
	case received := <-client.Send:
		assert.Equal(t, EventNotify, received.Event)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := &Hub{
		clients:     make(map[string]*Client),
		userClients: make(map[string]map[string]*Client),
		register:    make(chan *Client, 10),
		unregister:  make(chan *Client, 10),
		broadcast:   make(chan *WResponse, 10),
		usercast:    make(chan *userMessage, 10),
		stop:        make(chan struct{}),
	}

	go hub.Run()
	defer hub.Stop()

	client1 := &Client{
		ID:     "client-1",
		UserID: "user-1",
		Send:   make(chan *WResponse, 10),
	}
	client2 := &Client{
		ID:     "client-2",
		UserID: "user-2",
		Send:   make(chan *WResponse, 10),
	}

	hub.Register(client1)
	hub.Register(client2)
	time.Sleep(100 * time.Millisecond)

	msg := &WResponse{
		Event:     EventNotify,
		Code:      200,
		Data:      "broadcast message",
		Timestamp: time.Now().Unix(),
	}

	hub.Broadcast(msg)
	time.Sleep(100 * time.Millisecond)

	select {
	case <-client1.Send:
	case <-time.After(time.Second):
		t.Fatal("client1 did not receive message")
	}

	select {
	case <-client2.Send:
	case <-time.After(time.Second):
		t.Fatal("client2 did not receive message")
	}
}

func TestHubTryBroadcastIsNonBlockingAndRejectsStoppedHub(t *testing.T) {
	hub := &Hub{
		broadcast: make(chan *WResponse, 1),
		stop:      make(chan struct{}),
	}
	message := &WResponse{Event: EventAuthorization, Code: 200}

	assert.True(t, hub.TryBroadcast(message))
	assert.False(t, hub.TryBroadcast(message), "a full disposable queue must not block")
	assert.Same(t, message, <-hub.broadcast)

	close(hub.stop)
	assert.False(t, hub.TryBroadcast(message))
}

func TestAuthorizationRevisionResponseContainsOnlyDecimalRevision(t *testing.T) {
	assert.Nil(t, authorizationRevisionResponse(0))
	message := authorizationRevisionResponse(^uint64(0))
	if assert.NotNil(t, message) {
		assert.Equal(t, EventAuthorization, message.Event)
		assert.Equal(t, 200, message.Code)
		data, ok := message.Data.(gin.H)
		if assert.True(t, ok) {
			assert.Equal(t, "18446744073709551615", data["revision"])
			assert.Len(t, data, 1)
		}
	}
}

func TestGenerateClientID(t *testing.T) {
	id1 := GenerateClientID()
	id2 := GenerateClientID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}
