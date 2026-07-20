package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type WarehouseRepository struct{ db Querier }

func NewWarehouseRepository(db Querier) *WarehouseRepository { return &WarehouseRepository{db: db} }

const warehouseColumns = `id, user_id, name, address, contact_name, contact_phone, description, work_hours,
covered_area_m2, open_area_m2, available_covered_area_m2, available_open_area_m2, max_weight_kg, max_volume_m3,
services, consolidation_enabled, consolidation_min_volume_m3, consolidation_frequency, pickup_enabled, pickup_cities,
pickup_radius_km, own_transport, pickup_max_weight_kg, pickup_max_volume_m3, pickup_price_mode, dispatch_routes, status, created_at, updated_at`

func scanWarehouse(row pgx.Row) (*models.Warehouse, error) {
	var w models.Warehouse
	var address, pickupCities, dispatchRoutes []byte
	err := row.Scan(&w.ID, &w.UserID, &w.Name, &address, &w.ContactName, &w.ContactPhone, &w.Description, &w.WorkHours,
		&w.CoveredAreaM2, &w.OpenAreaM2, &w.AvailableCoveredAreaM2, &w.AvailableOpenAreaM2, &w.MaxWeightKg, &w.MaxVolumeM3,
		&w.Services, &w.ConsolidationEnabled, &w.ConsolidationMinVolumeM3, &w.ConsolidationFrequency, &w.PickupEnabled, &pickupCities,
		&w.PickupRadiusKm, &w.OwnTransport, &w.PickupMaxWeightKg, &w.PickupMaxVolumeM3, &w.PickupPriceMode, &dispatchRoutes, &w.Status, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(address, &w.Address); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(pickupCities, &w.PickupCities); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(dispatchRoutes, &w.DispatchRoutes); err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *WarehouseRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.Warehouse, error) {
	rows, err := r.db.Query(ctx, `SELECT `+warehouseColumns+` FROM warehouses WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.Warehouse, 0)
	for rows.Next() {
		w, err := scanWarehouse(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *w)
	}
	return items, rows.Err()
}

// SearchPublishedNear returns published warehouses whose address is within
// radiusKm of the given point, nearest first. address is JSONB, so lat/lng are
// extracted for the haversine distance.
func (r *WarehouseRepository) SearchPublishedNear(ctx context.Context, lat, lng, radiusKm float64) ([]models.Warehouse, error) {
	const q = `SELECT ` + warehouseColumns + ` FROM warehouses
		WHERE status = 'published'
		  AND haversine_km($1::float8, $2::float8, (address->>'lat')::float8, (address->>'lng')::float8) <= $3::float8
		ORDER BY haversine_km($1::float8, $2::float8, (address->>'lat')::float8, (address->>'lng')::float8) ASC`
	rows, err := r.db.Query(ctx, q, lat, lng, radiusKm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.Warehouse, 0)
	for rows.Next() {
		w, err := scanWarehouse(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *w)
	}
	return items, rows.Err()
}

// ListPublishedOwnersMatching returns distinct owner ids of published,
// pickup-enabled warehouses that can collect a cargo at (originLat, originLng)
// and dispatch toward (destLat, destLng): pickup within radius of the address
// OR any pickup city, and — if the warehouse declares dispatch routes — the
// destination near one of them. Used to notify matching warehouses.
func (r *WarehouseRepository) ListPublishedOwnersMatching(ctx context.Context, originLat, originLng, destLat, destLng, dispatchRadiusKm float64) ([]uuid.UUID, error) {
	const q = `
		SELECT DISTINCT w.user_id FROM warehouses w
		WHERE w.status = 'published' AND w.pickup_enabled = true
		  AND (
			haversine_km($1::float8, $2::float8, (w.address->>'lat')::float8, (w.address->>'lng')::float8) <= w.pickup_radius_km
			OR EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(NULLIF(w.pickup_cities, 'null'::jsonb), '[]'::jsonb)) pc
			           WHERE haversine_km($1::float8, $2::float8, (pc->>'lat')::float8, (pc->>'lng')::float8) <= w.pickup_radius_km)
		  )
		  AND (
			jsonb_array_length(COALESCE(NULLIF(w.dispatch_routes, 'null'::jsonb), '[]'::jsonb)) = 0
			OR EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(NULLIF(w.dispatch_routes, 'null'::jsonb), '[]'::jsonb)) dr
			           WHERE haversine_km($3::float8, $4::float8, (dr->'destination'->>'lat')::float8, (dr->'destination'->>'lng')::float8) <= $5::float8)
		  )
	`
	rows, err := r.db.Query(ctx, q, originLat, originLng, destLat, destLng, dispatchRadiusKm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetByID loads a warehouse without an owner check — for showing warehouse
// info (name/address/capacity) on offers to the cargo owner. Contacts are
// omitted at the service layer.
func (r *WarehouseRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Warehouse, error) {
	return scanWarehouse(r.db.QueryRow(ctx, `SELECT `+warehouseColumns+` FROM warehouses WHERE id=$1`, id))
}

// ListByIDs fetches warehouses for a set of ids in a single query, keyed by id.
// Used to avoid N+1 lookups when building offer views. Ids absent from the
// table are simply missing from the map.
func (r *WarehouseRepository) ListByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*models.Warehouse, error) {
	result := make(map[uuid.UUID]*models.Warehouse, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.db.Query(ctx, `SELECT `+warehouseColumns+` FROM warehouses WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		w, err := scanWarehouse(rows)
		if err != nil {
			return nil, err
		}
		result[w.ID] = w
	}
	return result, rows.Err()
}

func (r *WarehouseRepository) GetOwned(ctx context.Context, id, userID uuid.UUID) (*models.Warehouse, error) {
	return scanWarehouse(r.db.QueryRow(ctx, `SELECT `+warehouseColumns+` FROM warehouses WHERE id=$1 AND user_id=$2`, id, userID))
}

func (r *WarehouseRepository) Create(ctx context.Context, w *models.Warehouse) error {
	address, _ := json.Marshal(w.Address)
	pickup, _ := json.Marshal(w.PickupCities)
	dispatchRoutes, _ := json.Marshal(w.DispatchRoutes)
	_, err := r.db.Exec(ctx, `INSERT INTO warehouses (`+warehouseColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29)`,
		w.ID, w.UserID, w.Name, string(address), w.ContactName, w.ContactPhone, w.Description, w.WorkHours, w.CoveredAreaM2, w.OpenAreaM2, w.AvailableCoveredAreaM2, w.AvailableOpenAreaM2, w.MaxWeightKg, w.MaxVolumeM3, w.Services, w.ConsolidationEnabled, w.ConsolidationMinVolumeM3, w.ConsolidationFrequency, w.PickupEnabled, string(pickup), w.PickupRadiusKm, w.OwnTransport, w.PickupMaxWeightKg, w.PickupMaxVolumeM3, w.PickupPriceMode, string(dispatchRoutes), w.Status, w.CreatedAt, w.UpdatedAt)
	return err
}

func (r *WarehouseRepository) Update(ctx context.Context, w *models.Warehouse) error {
	address, _ := json.Marshal(w.Address)
	pickup, _ := json.Marshal(w.PickupCities)
	dispatchRoutes, _ := json.Marshal(w.DispatchRoutes)
	tag, err := r.db.Exec(ctx, `UPDATE warehouses SET name=$3,address=$4,contact_name=$5,contact_phone=$6,description=$7,work_hours=$8,covered_area_m2=$9,open_area_m2=$10,available_covered_area_m2=$11,available_open_area_m2=$12,max_weight_kg=$13,max_volume_m3=$14,services=$15,consolidation_enabled=$16,consolidation_min_volume_m3=$17,consolidation_frequency=$18,pickup_enabled=$19,pickup_cities=$20,pickup_radius_km=$21,own_transport=$22,pickup_max_weight_kg=$23,pickup_max_volume_m3=$24,pickup_price_mode=$25,dispatch_routes=$26,status=$27,updated_at=$28 WHERE id=$1 AND user_id=$2`,
		w.ID, w.UserID, w.Name, string(address), w.ContactName, w.ContactPhone, w.Description, w.WorkHours, w.CoveredAreaM2, w.OpenAreaM2, w.AvailableCoveredAreaM2, w.AvailableOpenAreaM2, w.MaxWeightKg, w.MaxVolumeM3, w.Services, w.ConsolidationEnabled, w.ConsolidationMinVolumeM3, w.ConsolidationFrequency, w.PickupEnabled, string(pickup), w.PickupRadiusKm, w.OwnTransport, w.PickupMaxWeightKg, w.PickupMaxVolumeM3, w.PickupPriceMode, string(dispatchRoutes), w.Status, w.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *WarehouseRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM warehouses WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
