package models

import (
	"time"

	"github.com/google/uuid"
)

type DriverCompetitionStatus string

const (
	DriverCompetitionOpen   DriverCompetitionStatus = "open"
	DriverCompetitionClosed DriverCompetitionStatus = "closed"
)

// DriverCompetition — конкурс склада на перевозку по направлению (ТЗ §11.4).
type DriverCompetition struct {
	ID           uuid.UUID               `json:"id"`
	WarehouseID  uuid.UUID               `json:"warehouse_id"`
	RouteID      uuid.UUID               `json:"route_id"`
	VolumeM3     float64                 `json:"volume_m3"`
	DispatchDate string                  `json:"dispatch_date"`
	Status       DriverCompetitionStatus `json:"status"`
	CreatedAt    time.Time               `json:"created_at"`
}

type DriverBidStatus string

const (
	DriverBidSubmitted DriverBidStatus = "submitted"
	DriverBidSelected  DriverBidStatus = "selected"
	DriverBidRejected  DriverBidStatus = "rejected"
	DriverBidWithdrawn DriverBidStatus = "withdrawn"
)

// DriverCompetitionBid — ценовое предложение водителя. Один водитель —
// одна ставка на конкурс.
type DriverCompetitionBid struct {
	ID            uuid.UUID       `json:"id"`
	CompetitionID uuid.UUID       `json:"competition_id"`
	DriverID      uuid.UUID       `json:"driver_id"`
	Price         float64         `json:"price"`
	Currency      string          `json:"currency"`
	Comment       string          `json:"comment"`
	Status        DriverBidStatus `json:"status"`
	CreatedAt     time.Time       `json:"created_at"`
}
