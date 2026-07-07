package models

import (
	"time"

	"github.com/google/uuid"
)

type VerificationStatus string

const (
	VerificationPending  VerificationStatus = "pending"
	VerificationApproved VerificationStatus = "approved"
	VerificationRejected VerificationStatus = "rejected"
)

type VerificationRequest struct {
	ID           uuid.UUID          `db:"id" json:"id"`
	UserID       uuid.UUID          `db:"user_id" json:"user_id"`
	Status       VerificationStatus `db:"status" json:"status"`
	RejectReason *string            `db:"reject_reason" json:"reject_reason,omitempty"`
	ReviewedBy   *uuid.UUID         `db:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time         `db:"reviewed_at" json:"reviewed_at,omitempty"`
	CreatedAt    time.Time          `db:"created_at" json:"created_at"`
}
