package models

import (
	"time"

	"github.com/google/uuid"
)

type WarehouseOfferStatus string

const (
	WarehouseOfferSubmitted WarehouseOfferStatus = "submitted"
	WarehouseOfferSelected  WarehouseOfferStatus = "selected"
	WarehouseOfferRejected  WarehouseOfferStatus = "rejected"
)

// WarehouseOffer is a warehouse's price bid to handle a cargo request
// (collect / consolidate / dispatch). The client picks one, which reveals the
// warehouse contact and opens a shared chat.
type WarehouseOffer struct {
	ID uuid.UUID `json:"id"`
	// Exactly one of these is set: an offer targets a single cargo request or a
	// consolidated request (a merged group of cargos).
	CargoRequestID        *uuid.UUID           `json:"cargo_request_id,omitempty"`
	ConsolidatedRequestID *uuid.UUID           `json:"consolidated_request_id,omitempty"`
	WarehouseID           uuid.UUID            `json:"warehouse_id"`
	WarehouseOwnerID      uuid.UUID            `json:"warehouse_owner_id"`
	Price                 float64              `json:"price"`
	Currency              string               `json:"currency"`
	Conditions            string               `json:"conditions"`
	Status                WarehouseOfferStatus `json:"status"`
	ChatID                *uuid.UUID           `json:"chat_id,omitempty"`
	CreatedAt             time.Time            `json:"created_at"`
	UpdatedAt             time.Time            `json:"updated_at"`
}
