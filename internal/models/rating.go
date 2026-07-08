package models

import (
	"time"

	"github.com/google/uuid"
)

type Rating struct {
	ID          uuid.UUID  `json:"id"`
	DealID      *uuid.UUID `json:"deal_id,omitempty"`
	RatedUserID uuid.UUID  `json:"rated_user_id"`
	RaterUserID uuid.UUID  `json:"rater_user_id"`
	Score       int        `json:"score"`
	Comment     *string    `json:"comment,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// WarehouseFillReport: self-reported warehouse load with an optional photo
// (MinIO key in PhotoURL; presigned view URLs are generated on read).
type WarehouseFillReport struct {
	ID                  uuid.UUID `json:"id"`
	UserID              uuid.UUID `json:"user_id"`
	ExpectedFillPercent float64   `json:"expected_fill_percent"`
	ActualFillPercent   float64   `json:"actual_fill_percent"`
	PhotoURL            *string   `json:"-"`
	ReportDate          time.Time `json:"report_date"`
	CreatedAt           time.Time `json:"created_at"`
}
