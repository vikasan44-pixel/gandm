package models

import (
	"time"

	"github.com/google/uuid"
)

type ConsolidationStatus string

const (
	ConsolidationSuggested  ConsolidationStatus = "suggested"
	ConsolidationAAgreed    ConsolidationStatus = "a_agreed"
	ConsolidationBAgreed    ConsolidationStatus = "b_agreed"
	ConsolidationBothAgreed ConsolidationStatus = "both_agreed"
	ConsolidationDeclined   ConsolidationStatus = "declined"
)

type ConsolidationSuggestion struct {
	ID             uuid.UUID           `json:"id"`
	CargoRequestA  uuid.UUID           `json:"cargo_request_a"`
	CargoRequestB  uuid.UUID           `json:"cargo_request_b"`
	DirectionLabel string              `json:"direction_label"`
	Status         ConsolidationStatus `json:"status"`
	CreatedAt      time.Time           `json:"created_at"`
}

type ConsolidatedInviteStatus string

const (
	InviteNone     ConsolidatedInviteStatus = "none"
	InviteInvited  ConsolidatedInviteStatus = "invited"
	InviteAccepted ConsolidatedInviteStatus = "accepted"
)

// ConsolidatedRequest reuses CargoRequestStatus — same open/matched/closed
// lifecycle. Coordinates carry label+country like regular cargo points
// (labels for humans, countries for the per-country matching radius).
// Invite fields drive the paid unlock flow: initiator (subscribed) invites,
// the other client accepts after paying or via their own subscription.
type ConsolidatedRequest struct {
	ID                uuid.UUID                `json:"id"`
	Origin            GeoPoint                 `json:"origin"`
	Destination       GeoPoint                 `json:"destination"`
	TotalVolumeM3     float64                  `json:"total_volume_m3"`
	TotalWeightKg     float64                  `json:"total_weight_kg"`
	MemberRequestIDs  []uuid.UUID              `json:"member_request_ids"`
	Status            CargoRequestStatus       `json:"status"`
	InviteStatus      ConsolidatedInviteStatus `json:"invite_status"`
	InitiatorClientID *uuid.UUID               `json:"initiator_client_id,omitempty"`
	InvitedClientID   *uuid.UUID               `json:"invited_client_id,omitempty"`
	ChatID            *uuid.UUID               `json:"chat_id,omitempty"`
	CreatedAt         time.Time                `json:"created_at"`
}

type ConsolidatedPayment struct {
	ID                    uuid.UUID `json:"id"`
	ConsolidatedRequestID uuid.UUID `json:"consolidated_request_id"`
	ClientID              uuid.UUID `json:"client_id"`
	Provider              string    `json:"provider"`
	ProviderRef           string    `json:"provider_ref"`
	CreatedAt             time.Time `json:"created_at"`
}

type ConsolidatedSelection struct {
	ConsolidatedRequestID uuid.UUID `json:"consolidated_request_id"`
	ClientID              uuid.UUID `json:"client_id"`
	OfferID               uuid.UUID `json:"offer_id"`
	CreatedAt             time.Time `json:"created_at"`
}
