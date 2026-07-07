package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID           uuid.UUID       `db:"id" json:"id"`
	AdminID      uuid.UUID       `db:"admin_id" json:"admin_id"`
	Action       string          `db:"action" json:"action"`
	TargetUserID *uuid.UUID      `db:"target_user_id" json:"target_user_id,omitempty"`
	Details      json.RawMessage `db:"details" json:"details,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
}
