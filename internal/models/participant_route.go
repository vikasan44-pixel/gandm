package models

import (
	"time"

	"github.com/google/uuid"
)

type ParticipantRoute struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Origin      GeoPoint  `json:"origin"`
	Destination GeoPoint  `json:"destination"`
	CreatedAt   time.Time `json:"created_at"`
}
