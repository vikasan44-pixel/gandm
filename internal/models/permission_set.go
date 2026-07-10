package models

import "github.com/google/uuid"

type PermissionSet struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	// PriceKZT — цена тарифа в тенге за месяц (ТЗ §19.5); 0 = бесплатный.
	// Информационная до подключения реального платёжного провайдера.
	PriceKZT float64 `db:"price_kzt" json:"price_kzt"`
}
