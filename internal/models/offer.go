package models

import (
	"time"

	"github.com/google/uuid"
)

type OfferStatus string

const (
	OfferSubmitted OfferStatus = "submitted"
	OfferSelected  OfferStatus = "selected"
	OfferRejected  OfferStatus = "rejected"
)

// Offer targets exactly one of: a single cargo request or a consolidated
// request (enforced by a DB CHECK constraint).
type Offer struct {
	ID                    uuid.UUID   `json:"id"`
	CargoRequestID        *uuid.UUID  `json:"cargo_request_id,omitempty"`
	ConsolidatedRequestID *uuid.UUID  `json:"consolidated_request_id,omitempty"`
	ParticipantID         uuid.UUID   `json:"participant_id"`
	Price                 float64     `json:"price"`
	Currency              string      `json:"currency"`
	Conditions            string      `json:"conditions"`
	WarehouseFillPercent  *float64    `json:"warehouse_fill_percent,omitempty"`
	Status                OfferStatus `json:"status"`
	CreatedAt             time.Time   `json:"created_at"`
}
