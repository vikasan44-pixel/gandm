package models

import (
	"time"

	"github.com/google/uuid"
)

type ContactReveal struct {
	ID             uuid.UUID `json:"id"`
	ClientID       uuid.UUID `json:"client_id"`
	ParticipantID  uuid.UUID `json:"participant_id"`
	CargoRequestID uuid.UUID `json:"cargo_request_id"`
	IsPaid         bool      `json:"is_paid"`
	CreatedAt      time.Time `json:"created_at"`
}

type Chat struct {
	ID             uuid.UUID `json:"id"`
	CargoRequestID uuid.UUID `json:"cargo_request_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type Message struct {
	ID            uuid.UUID `json:"id"`
	ChatID        uuid.UUID `json:"chat_id"`
	SenderID      uuid.UUID `json:"sender_id"`
	Body          string    `json:"body"`
	AttachmentURL *string   `json:"attachment_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
