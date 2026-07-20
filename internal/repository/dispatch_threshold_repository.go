package repository

import (
	"context"

	"github.com/google/uuid"

	"gandm/internal/models"
)

type DispatchThresholdRepository struct {
	db Querier
}

func NewDispatchThresholdRepository(db Querier) *DispatchThresholdRepository {
	return &DispatchThresholdRepository{db: db}
}

func (r *DispatchThresholdRepository) Upsert(ctx context.Context, t *models.DispatchThreshold) error {
	const q = `
		INSERT INTO warehouse_dispatch_thresholds (
			route_id, warehouse_id, threshold_m3, accrued_m3,
			estimated_dispatch_date, status, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (route_id) DO UPDATE
		SET warehouse_id = EXCLUDED.warehouse_id,
			threshold_m3 = EXCLUDED.threshold_m3,
			accrued_m3 = EXCLUDED.accrued_m3,
			estimated_dispatch_date = EXCLUDED.estimated_dispatch_date,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.Exec(ctx, q, t.RouteID, t.WarehouseID, t.ThresholdM3, t.ManualAccruedM3,
		t.EstimatedDispatchDate, t.Status, t.UpdatedAt)
	return err
}

func (r *DispatchThresholdRepository) DeleteByRouteID(ctx context.Context, routeID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM warehouse_dispatch_thresholds WHERE route_id = $1`, routeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByUserID returns the user's thresholds keyed by route id.
func (r *DispatchThresholdRepository) ListByUserID(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]models.DispatchThreshold, error) {
	const q = `
		SELECT t.route_id, t.warehouse_id, t.threshold_m3, t.accrued_m3,
		       t.estimated_dispatch_date, t.status, t.updated_at
		FROM warehouse_dispatch_thresholds t
		JOIN participant_routes pr ON pr.id = t.route_id
		WHERE pr.user_id = $1
	`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make(map[uuid.UUID]models.DispatchThreshold)
	for rows.Next() {
		var t models.DispatchThreshold
		if err := rows.Scan(&t.RouteID, &t.WarehouseID, &t.ThresholdM3, &t.AccruedM3,
			&t.EstimatedDispatchDate, &t.Status, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.ManualAccruedM3 = t.AccruedM3
		t.RemainingM3 = max(0, t.ThresholdM3-t.AccruedM3)
		items[t.RouteID] = t
	}
	return items, rows.Err()
}

// ForUsersMatchingRoute picks, per user, the threshold of their route that
// matches the given cargo endpoints (same per-country radius rule as the
// available-cargo queries — if that SQL changes, change this too). Used to
// enrich anonymized offers with «сколько осталось до отправки» without
// revealing whose warehouse it is.
func (r *DispatchThresholdRepository) ForUsersMatchingRoute(ctx context.Context, userIDs []uuid.UUID, origin, destination models.GeoPoint, cnKm, kzKm float64) (map[uuid.UUID]models.DispatchThreshold, error) {
	if len(userIDs) == 0 {
		return map[uuid.UUID]models.DispatchThreshold{}, nil
	}
	const q = `
		SELECT DISTINCT ON (pr.user_id) pr.user_id, t.route_id, t.warehouse_id,
		       t.threshold_m3, t.accrued_m3,
		       COALESCE((
		           SELECT SUM(cr.volume_m3)
		           FROM cargo_requests cr
		           WHERE cr.client_id = pr.user_id
		             AND cr.status IN ('open', 'matched')
		             AND haversine_km(pr.origin_lat, pr.origin_lng, cr.origin_lat, cr.origin_lng)
		                 <= GREATEST(
		                      CASE WHEN pr.origin_country = 'cn' THEN $6::float8 ELSE $7::float8 END,
		                      CASE WHEN cr.origin_country = 'cn' THEN $6::float8 ELSE $7::float8 END)
		             AND haversine_km(pr.destination_lat, pr.destination_lng, cr.destination_lat, cr.destination_lng)
		                 <= GREATEST(
		                      CASE WHEN pr.destination_country = 'cn' THEN $6::float8 ELSE $7::float8 END,
		                      CASE WHEN cr.destination_country = 'cn' THEN $6::float8 ELSE $7::float8 END)
		       ), 0)::float8 AS platform_accrued_m3,
		       t.estimated_dispatch_date,
		       t.status, t.updated_at
		FROM warehouse_dispatch_thresholds t
		JOIN participant_routes pr ON pr.id = t.route_id
		WHERE pr.user_id = ANY($1)
		  AND t.status IN ('collecting', 'ready')
		  AND haversine_km(pr.origin_lat, pr.origin_lng, $2, $3)
		      <= GREATEST(
		           CASE WHEN pr.origin_country = 'cn' THEN $6::float8 ELSE $7::float8 END,
		           CASE WHEN $8 = 'cn' THEN $6::float8 ELSE $7::float8 END)
		  AND haversine_km(pr.destination_lat, pr.destination_lng, $4, $5)
		      <= GREATEST(
		           CASE WHEN pr.destination_country = 'cn' THEN $6::float8 ELSE $7::float8 END,
		           CASE WHEN $9 = 'cn' THEN $6::float8 ELSE $7::float8 END)
		ORDER BY pr.user_id, t.updated_at DESC
	`
	rows, err := r.db.Query(ctx, q, userIDs,
		origin.Lat, origin.Lng, destination.Lat, destination.Lng,
		cnKm, kzKm, origin.Country, destination.Country)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make(map[uuid.UUID]models.DispatchThreshold)
	for rows.Next() {
		var userID uuid.UUID
		var t models.DispatchThreshold
		if err := rows.Scan(&userID, &t.RouteID, &t.WarehouseID, &t.ThresholdM3,
			&t.ManualAccruedM3, &t.PlatformAccruedM3, &t.EstimatedDispatchDate, &t.Status, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.AccruedM3 = t.ManualAccruedM3 + t.PlatformAccruedM3
		t.RemainingM3 = max(0, t.ThresholdM3-t.AccruedM3)
		if t.Status == "collecting" || t.Status == "ready" {
			if t.AccruedM3 >= t.ThresholdM3 {
				t.Status = "ready"
			} else {
				t.Status = "collecting"
			}
		}
		items[userID] = t
	}
	return items, rows.Err()
}
