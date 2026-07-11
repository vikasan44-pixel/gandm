package models

import (
	"time"

	"github.com/google/uuid"
)

// Vehicle is one unit of a participant's fleet (ТЗ §11.1). BodyType is free
// text on purpose — the set of body types is business data, not schema.
type Vehicle struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	Axles      int       `json:"axles"`
	CapacityKg float64   `json:"capacity_kg"`
	CapacityM3 float64   `json:"capacity_m3"`
	LengthM    float64   `json:"length_m"`
	WidthM     float64   `json:"width_m"`
	HeightM    float64   `json:"height_m"`
	BodyType   string    `json:"body_type"`
	// Местонахождение координатами (по карте), опционально — играет роль
	// «откуда» в публичном поиске по направлению. nil, если не указано.
	Location *GeoPoint `json:"location,omitempty"`
	// Куда машина готова везти — ноль или несколько назначений (координатами).
	Destinations []VehicleDestination `json:"destinations"`
	CreatedAt    time.Time            `json:"created_at"`
}

// VehicleDestination — одно из назначений машины. ID нужен, чтобы удалять
// конкретное назначение; Point — координаты (как груз/маршруты).
type VehicleDestination struct {
	ID    uuid.UUID `json:"id"`
	Point GeoPoint  `json:"point"`
}
