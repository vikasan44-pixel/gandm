package models

import (
	"time"

	"github.com/google/uuid"
)

type CustomsOfferStatus string

const (
	CustomsOfferSubmitted CustomsOfferStatus = "submitted"
	CustomsOfferSelected  CustomsOfferStatus = "selected"
	CustomsOfferRejected  CustomsOfferStatus = "rejected"
	CustomsOfferWithdrawn CustomsOfferStatus = "withdrawn"
)

// ConsolidatedCustomsOffer is a customs representative's bid for clearing a
// matched consolidated shipment (ТЗ §10.2). One rep — one offer per
// consolidation.
type ConsolidatedCustomsOffer struct {
	ID                    uuid.UUID          `json:"id"`
	ConsolidatedRequestID uuid.UUID          `json:"consolidated_request_id"`
	CustomsRepID          uuid.UUID          `json:"customs_rep_id"`
	Price                 float64            `json:"price"`
	Currency              string             `json:"currency"`
	Conditions            string             `json:"conditions"`
	Status                CustomsOfferStatus `json:"status"`
	CreatedAt             time.Time          `json:"created_at"`
}
