package models

import (
	"time"

	"github.com/google/uuid"
)

type AdminRole string

const (
	AdminRoleAdmin     AdminRole = "admin"
	AdminRoleModerator AdminRole = "moderator"
)

type Admin struct {
	ID           uuid.UUID `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	Role         AdminRole `db:"role" json:"role"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}
