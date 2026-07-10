package models

import (
	"time"

	"github.com/google/uuid"
)

// WarehouseBatch — партия склада (ТЗ §10.1): грузы, набранные к отправке
// по направлению. Открывается при достижении порога отправки, закрывается
// (DispatchedAt), когда склад сбрасывает набранный объём ниже порога.
type WarehouseBatch struct {
	ID           uuid.UUID  `json:"id"`
	WarehouseID  uuid.UUID  `json:"warehouse_id"`
	RouteID      uuid.UUID  `json:"route_id"`
	VolumeM3     float64    `json:"volume_m3"`
	DispatchedAt *time.Time `json:"dispatched_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}
