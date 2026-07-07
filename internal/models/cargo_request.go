package models

import (
	"time"

	"github.com/google/uuid"
)

type CargoRequestStatus string

const (
	CargoRequestOpen    CargoRequestStatus = "open"
	CargoRequestMatched CargoRequestStatus = "matched"
	CargoRequestClosed  CargoRequestStatus = "closed"
)

type CargoRequest struct {
	ID          uuid.UUID          `json:"id"`
	ClientID    uuid.UUID          `json:"client_id"`
	Origin      GeoPoint           `json:"origin"`
	Destination GeoPoint           `json:"destination"`
	VolumeM3    float64            `json:"volume_m3"`
	WeightKg    float64            `json:"weight_kg"`
	Description string             `json:"description"`
	Status      CargoRequestStatus `json:"status"`
	CreatedAt   time.Time          `json:"created_at"`
}
