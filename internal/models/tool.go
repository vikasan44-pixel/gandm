package models

import "github.com/google/uuid"

type Tool struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Key         string    `db:"key" json:"key"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	Category    string    `db:"category" json:"category"`
	IsActive    bool      `db:"is_active" json:"is_active"`
	// PriceKZT — цена инструмента, ₸/мес; 0 = бесплатный. Задаётся админом.
	PriceKZT float64 `db:"price_kzt" json:"price_kzt"`
}
