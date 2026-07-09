package models

import (
	"time"

	"github.com/google/uuid"
)

// DispatchThreshold is a warehouse's «порог отправки» for one of its routes
// (ТЗ §5.2): dispatch happens once AccruedM3 reaches ThresholdM3. Accrued is
// self-reported by the warehouse for now.
type DispatchThreshold struct {
	RouteID     uuid.UUID `json:"route_id"`
	ThresholdM3 float64   `json:"threshold_m3"`
	AccruedM3   float64   `json:"accrued_m3"`
	UpdatedAt   time.Time `json:"updated_at"`
}
