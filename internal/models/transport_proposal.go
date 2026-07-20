package models

import (
	"time"

	"github.com/google/uuid"
)

// TransportProposalStatus is the negotiation state machine:
// sent → carrier_quoted → client_countered → carrier_final → agreed | rejected.
type TransportProposalStatus string

const (
	// ProposalSent — client sent the cargo, waiting for the carrier's price.
	ProposalSent TransportProposalStatus = "sent"
	// ProposalCarrierQuoted — carrier named a price, waiting for the client.
	ProposalCarrierQuoted TransportProposalStatus = "carrier_quoted"
	// ProposalClientCountered — client proposed a different price, carrier's turn.
	ProposalClientCountered TransportProposalStatus = "client_countered"
	// ProposalCarrierFinal — carrier's take-it-or-leave-it price, client's turn.
	ProposalCarrierFinal TransportProposalStatus = "carrier_final"
	// ProposalAgreed — both sides agreed; contacts revealed, chat opened.
	ProposalAgreed TransportProposalStatus = "agreed"
	// ProposalRejected — negotiation ended without a deal.
	ProposalRejected TransportProposalStatus = "rejected"
)

// PriceParty identifies who set the current price.
const (
	PricePartyCarrier = "carrier"
	PricePartyClient  = "client"
)

// TransportProposalItem is one package ("место") with its dimensions.
type TransportProposalItem struct {
	ID       uuid.UUID `json:"id"`
	Position int       `json:"position"`
	LengthM  float64   `json:"length_m"`
	WidthM   float64   `json:"width_m"`
	HeightM  float64   `json:"height_m"`
}

type TransportProposal struct {
	ID             uuid.UUID  `json:"id"`
	ClientID       uuid.UUID  `json:"client_id"`
	VehicleID      uuid.UUID  `json:"vehicle_id"`
	CarrierID      uuid.UUID  `json:"carrier_id"`
	CargoRequestID *uuid.UUID `json:"cargo_request_id,omitempty"`

	Origin      GeoPoint `json:"origin"`
	Destination GeoPoint `json:"destination"`

	CargoName   string  `json:"cargo_name"`
	VolumeM3    float64 `json:"volume_m3"`
	WeightKg    float64 `json:"weight_kg"`
	PlacesCount int     `json:"places_count"`
	PickupDate  string  `json:"pickup_date"`

	Status       TransportProposalStatus `json:"status"`
	CurrentPrice *float64                `json:"current_price,omitempty"`
	LastPriceBy  string                  `json:"last_price_by,omitempty"`
	Currency     string                  `json:"currency"`

	ChatID    *uuid.UUID `json:"chat_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	Items []TransportProposalItem `json:"items"`
}
