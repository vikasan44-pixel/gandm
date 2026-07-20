package models

import (
	"time"

	"github.com/google/uuid"
)

// DispatchThreshold is a warehouse's «порог отправки» for one of its routes
// (ТЗ §5.2): dispatch happens once AccruedM3 reaches ThresholdM3.
// AccruedM3 is the effective total: active cargo from "Мои заявки" plus
// cargo collected outside the platform and entered by the warehouse.
type DispatchThreshold struct {
	RouteID               uuid.UUID  `json:"route_id"`
	WarehouseID           *uuid.UUID `json:"warehouse_id,omitempty"`
	ThresholdM3           float64    `json:"threshold_m3"`
	AccruedM3             float64    `json:"accrued_m3"`
	PlatformAccruedM3     float64    `json:"platform_accrued_m3"`
	ManualAccruedM3       float64    `json:"manual_accrued_m3"`
	RemainingM3           float64    `json:"remaining_m3"`
	EstimatedDispatchDate *time.Time `json:"estimated_dispatch_date,omitempty"`
	Status                string     `json:"status"`
	UpdatedAt             time.Time  `json:"updated_at"`
}
