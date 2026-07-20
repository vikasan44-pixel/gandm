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

// ConsolidationSuggestion — предложение объединиться ГРУППЕ клиентов (ТЗ
// §4.2 «два клиента и более»). Участники и их ответы живут в
// consolidation_suggestion_members; статусы a_agreed/b_agreed — legacy
// парной схемы, новые предложения проходят suggested → both_agreed
// (группа собрана) | declined (собирать некого).
type ConsolidationSuggestion struct {
	ID             uuid.UUID           `json:"id"`
	DirectionLabel string              `json:"direction_label"`
	Status         ConsolidationStatus `json:"status"`
	CreatedAt      time.Time           `json:"created_at"`
	// ResolvesAt is when the response window closes; after it the suggestion is
	// resolved with whoever agreed even if others never answered.
	ResolvesAt time.Time `json:"resolves_at"`
}

type SuggestionResponse string

const (
	SuggestionPending  SuggestionResponse = "pending"
	SuggestionAgreed   SuggestionResponse = "agreed"
	SuggestionDeclined SuggestionResponse = "declined"
)

// SuggestionMember — одна заявка в групповом предложении со своим ответом.
type SuggestionMember struct {
	SuggestionID   uuid.UUID          `json:"suggestion_id"`
	CargoRequestID uuid.UUID          `json:"cargo_request_id"`
	ClientID       uuid.UUID          `json:"client_id"`
	Response       SuggestionResponse `json:"response"`
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
