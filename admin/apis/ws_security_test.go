package apis

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/center/websocket"
)

func canonicalWebSocketTestTicket(fill byte) string {
	return base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{fill}, 32))
}

func TestWebSocketTicketFromProtocolsNeverUsesURLCredential(t *testing.T) {
	ticket := canonicalWebSocketTestTicket(1)
	request := httptest.NewRequest(http.MethodGet, "/admin/api/ws/connect?ticket=url-secret", nil)
	request.Header.Set("Sec-WebSocket-Protocol", websocket.ApplicationSubprotocol+", "+websocket.TicketProtocolPrefix+ticket)
	got, ok := webSocketTicketFromProtocols(request)
	if !ok || got != ticket {
		t.Fatalf("webSocketTicketFromProtocols() = (%q, %v)", got, ok)
	}
	if got == request.URL.Query().Get("ticket") {
		t.Fatal("protocol parser accepted the URL credential")
	}
}

func TestWebSocketTicketFromProtocolsRejectsMalformedOrAmbiguousInput(t *testing.T) {
	ticketOne := canonicalWebSocketTestTicket(1)
	ticketTwo := canonicalWebSocketTestTicket(2)
	tests := []string{
		websocket.ApplicationSubprotocol,
		websocket.TicketProtocolPrefix + ticketOne,
		websocket.ApplicationSubprotocol + ", " + websocket.TicketProtocolPrefix,
		websocket.ApplicationSubprotocol + ", " + websocket.TicketProtocolPrefix + strings.Repeat("a", 42) + ".",
		websocket.ApplicationSubprotocol + ", " + websocket.TicketProtocolPrefix + ticketOne + ", " + websocket.TicketProtocolPrefix + ticketTwo,
	}
	for _, protocols := range tests {
		request := httptest.NewRequest(http.MethodGet, "/admin/api/ws/connect", nil)
		request.Header.Set("Sec-WebSocket-Protocol", protocols)
		if ticket, ok := webSocketTicketFromProtocols(request); ok || ticket != "" {
			t.Fatalf("protocols %q parsed as (%q, %v)", protocols, ticket, ok)
		}
	}
}
