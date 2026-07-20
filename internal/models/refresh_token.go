package models

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken is one row of the server-side refresh-token registry. A refresh
// JWT is only accepted while its jti has a matching, un-revoked, unexpired row
// here — that is what makes refresh tokens revocable and rotatable.
type RefreshToken struct {
	JTI         uuid.UUID
	SubjectID   uuid.UUID
	SubjectType string
	IssuedAt    time.Time
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	ReplacedBy  *uuid.UUID
}
