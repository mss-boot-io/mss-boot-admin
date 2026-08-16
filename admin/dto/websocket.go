package dto

import "time"

type WebSocketTicketResponse struct {
	Ticket    string    `json:"ticket"`
	Protocol  string    `json:"protocol"`
	ExpiresAt time.Time `json:"expiresAt"`
}
