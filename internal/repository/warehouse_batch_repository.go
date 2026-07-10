package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
)

type WarehouseBatchRepository struct {
	db Querier
}

func NewWarehouseBatchRepository(db Querier) *WarehouseBatchRepository {
	return &WarehouseBatchRepository{db: db}
}

func (r *WarehouseBatchRepository) Create(ctx context.Context, b *models.WarehouseBatch) error {
	const q = `
		INSERT INTO warehouse_batches (id, warehouse_id, route_id, volume_m3, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, q, b.ID, b.WarehouseID, b.RouteID, b.VolumeM3, b.CreatedAt)
	return err
}

// HasActiveForRoute: активная (не отправленная) партия уже есть — повторные
// сохранения порога не плодят дубликаты чатов.
func (r *WarehouseBatchRepository) HasActiveForRoute(ctx context.Context, routeID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM warehouse_batches WHERE route_id = $1 AND dispatched_at IS NULL)`
	var ok bool
	err := r.db.QueryRow(ctx, q, routeID).Scan(&ok)
	return ok, err
}

// DispatchActiveForRoute закрывает активную партию направления (склад
// сбросил набранный объём ниже порога = отправил). Отсутствие активной
// партии — не ошибка.
func (r *WarehouseBatchRepository) DispatchActiveForRoute(ctx context.Context, routeID uuid.UUID, at time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE warehouse_batches SET dispatched_at = $2 WHERE route_id = $1 AND dispatched_at IS NULL`, routeID, at)
	return err
}

func (r *WarehouseBatchRepository) AddMember(ctx context.Context, batchID, cargoRequestID, clientID uuid.UUID) error {
	const q = `
		INSERT INTO warehouse_batch_members (batch_id, cargo_request_id, client_id)
		VALUES ($1, $2, $3) ON CONFLICT DO NOTHING
	`
	_, err := r.db.Exec(ctx, q, batchID, cargoRequestID, clientID)
	return err
}

// BatchCandidate — груз, который поедет в партии: matched-сделка этого
// склада, направление которой совпадает с маршрутом партии по радиусу.
type BatchCandidate struct {
	CargoRequestID uuid.UUID
	ClientID       uuid.UUID
	Description    string
}

// ListBatchCandidates: matched-сделки склада (contact_reveals) по
// направлению маршрута (та же радиусная логика, что и в матчинге), ещё не
// входившие ни в одну партию. Направление сравнивается точка-к-точке:
// origin груза к origin маршрута, destination к destination.
func (r *WarehouseBatchRepository) ListBatchCandidates(ctx context.Context, warehouseID, routeID uuid.UUID, cnKm, kzKm float64) ([]BatchCandidate, error) {
	const q = `
		SELECT cr.id, cr.client_id, cr.description
		FROM cargo_requests cr
		JOIN contact_reveals rev ON rev.cargo_request_id = cr.id AND rev.participant_id = $1
		JOIN participant_routes pr ON pr.id = $2
		WHERE cr.status = 'matched'
		  AND NOT EXISTS (SELECT 1 FROM warehouse_batch_members m WHERE m.cargo_request_id = cr.id)
		  AND haversine_km(cr.origin_lat, cr.origin_lng, pr.origin_lat, pr.origin_lng)
		      <= GREATEST(
		           CASE WHEN cr.origin_country = 'cn' THEN $3::float8 ELSE $4::float8 END,
		           CASE WHEN pr.origin_country = 'cn' THEN $3::float8 ELSE $4::float8 END)
		  AND haversine_km(cr.destination_lat, cr.destination_lng, pr.destination_lat, pr.destination_lng)
		      <= GREATEST(
		           CASE WHEN cr.destination_country = 'cn' THEN $3::float8 ELSE $4::float8 END,
		           CASE WHEN pr.destination_country = 'cn' THEN $3::float8 ELSE $4::float8 END)
		ORDER BY cr.created_at ASC
	`
	rows, err := r.db.Query(ctx, q, warehouseID, routeID, cnKm, kzKm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]BatchCandidate, 0)
	for rows.Next() {
		var c BatchCandidate
		if err := rows.Scan(&c.CargoRequestID, &c.ClientID, &c.Description); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
