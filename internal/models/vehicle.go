package models

import (
	"time"

	"github.com/google/uuid"
)

// Vehicle is one unit of a participant's fleet (ТЗ §11.1). BodyType is free
// text on purpose — the set of body types is business data, not schema.
type Vehicle struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	Axles           int       `json:"axles"`
	CapacityKg      float64   `json:"capacity_kg"`
	LengthM         float64   `json:"length_m"`
	WidthM          float64   `json:"width_m"`
	HeightM         float64   `json:"height_m"`
	BodyType        string    `json:"body_type"`
	CurrentLocation string    `json:"current_location"`
	// Опциональное объявленное направление «готов везти откуда → куда»,
	// координатами (как груз/маршруты) — для публичного поиска по радиусу.
	// nil, если направление не указано.
	ReadyOrigin      *GeoPoint `json:"ready_origin,omitempty"`
	ReadyDestination *GeoPoint `json:"ready_destination,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}
