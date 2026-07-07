package models

import "github.com/google/uuid"

type Tool struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Key         string    `db:"key" json:"key"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	Category    string    `db:"category" json:"category"`
	IsActive    bool      `db:"is_active" json:"is_active"`
}
