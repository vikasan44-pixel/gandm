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

// ConsolidatedRequest reuses CargoRequestStatus — same open/matched/closed
// lifecycle. Coordinates carry label+country like regular cargo points
// (labels for humans, countries for the per-country matching radius).
type ConsolidatedRequest struct {
	ID               uuid.UUID          `json:"id"`
	Origin           GeoPoint           `json:"origin"`
	Destination      GeoPoint           `json:"destination"`
	TotalVolumeM3    float64            `json:"total_volume_m3"`
	TotalWeightKg    float64            `json:"total_weight_kg"`
	MemberRequestIDs []uuid.UUID        `json:"member_request_ids"`
	Status           CargoRequestStatus `json:"status"`
	CreatedAt        time.Time          `json:"created_at"`
}
