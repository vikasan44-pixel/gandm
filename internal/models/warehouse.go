package models

import (
	"time"

	"github.com/google/uuid"
)

type WarehouseStatus string

const (
	WarehouseDraft     WarehouseStatus = "draft"
	WarehousePublished WarehouseStatus = "published"
	WarehousePaused    WarehouseStatus = "paused"
)

type WarehouseDispatchRoute struct {
	ID          uuid.UUID `json:"id"`
	Origin      GeoPoint  `json:"origin"`
	Destination GeoPoint  `json:"destination"`
}

type Warehouse struct {
	ID                       uuid.UUID                `json:"id"`
	UserID                   uuid.UUID                `json:"user_id"`
	Name                     string                   `json:"name"`
	Address                  GeoPoint                 `json:"address"`
	ContactName              string                   `json:"contact_name"`
	ContactPhone             string                   `json:"contact_phone"`
	Description              string                   `json:"description"`
	WorkHours                string                   `json:"work_hours"`
	CoveredAreaM2            float64                  `json:"covered_area_m2"`
	OpenAreaM2               float64                  `json:"open_area_m2"`
	AvailableCoveredAreaM2   float64                  `json:"available_covered_area_m2"`
	AvailableOpenAreaM2      float64                  `json:"available_open_area_m2"`
	MaxWeightKg              float64                  `json:"max_weight_kg"`
	MaxVolumeM3              float64                  `json:"max_volume_m3"`
	Services                 []string                 `json:"services"`
	ConsolidationEnabled     bool                     `json:"consolidation_enabled"`
	ConsolidationMinVolumeM3 float64                  `json:"consolidation_min_volume_m3"`
	ConsolidationFrequency   string                   `json:"consolidation_frequency"`
	PickupEnabled            bool                     `json:"pickup_enabled"`
	PickupCities             []GeoPoint               `json:"pickup_cities"`
	PickupRadiusKm           float64                  `json:"pickup_radius_km"`
	OwnTransport             bool                     `json:"own_transport"`
	PickupMaxWeightKg        float64                  `json:"pickup_max_weight_kg"`
	PickupMaxVolumeM3        float64                  `json:"pickup_max_volume_m3"`
	PickupPriceMode          string                   `json:"pickup_price_mode"`
	DispatchRoutes           []WarehouseDispatchRoute `json:"dispatch_routes"`
	Status                   WarehouseStatus          `json:"status"`
	CreatedAt                time.Time                `json:"created_at"`
	UpdatedAt                time.Time                `json:"updated_at"`
}
