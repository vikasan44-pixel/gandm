package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type CargoRequestRepository struct {
	db Querier
}

func NewCargoRequestRepository(db Querier) *CargoRequestRepository {
	return &CargoRequestRepository{db: db}
}

const cargoRequestColumns = `id, client_id, origin_lat, origin_lng, origin_label, origin_source, origin_country, origin_labels, destination_lat, destination_lng, destination_label, destination_source, destination_country, destination_labels, volume_m3, weight_kg, category, description, status, created_at, packaging, places_count, stackable, adr_required`

func scanCargoRequestFields(row pgx.Row, c *models.CargoRequest) error {
	var originLabels, destinationLabels []byte
	err := row.Scan(
		&c.ID, &c.ClientID,
		&c.Origin.Lat, &c.Origin.Lng, &c.Origin.Label, &c.Origin.Source, &c.Origin.Country, &originLabels,
		&c.Destination.Lat, &c.Destination.Lng, &c.Destination.Label, &c.Destination.Source, &c.Destination.Country, &destinationLabels,
		&c.VolumeM3, &c.WeightKg, &c.Category, &c.Description, &c.Status, &c.CreatedAt,
		&c.Packaging, &c.PlacesCount, &c.Stackable, &c.ADRRequired,
	)
	if err != nil {
		return err
	}
	c.Origin.Labels = scanLabels(originLabels)
	c.Destination.Labels = scanLabels(destinationLabels)
	return nil
}

// attachCargoItems loads the per-place dimensions for a set of cargo requests
// in one query and groups them onto each request.
func attachCargoItems(ctx context.Context, db Querier, cargos []models.CargoRequest) error {
	if len(cargos) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(cargos))
	idx := make(map[uuid.UUID]int, len(cargos))
	for i := range cargos {
		ids[i] = cargos[i].ID
		idx[cargos[i].ID] = i
	}
	const q = `SELECT cargo_request_id, id, position, length_m, width_m, height_m FROM cargo_request_items WHERE cargo_request_id = ANY($1) ORDER BY position ASC`
	rows, err := db.Query(ctx, q, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid uuid.UUID
		var it models.CargoRequestItem
		if err := rows.Scan(&cid, &it.ID, &it.Position, &it.LengthM, &it.WidthM, &it.HeightM); err != nil {
			return err
		}
		if i, ok := idx[cid]; ok {
			cargos[i].Items = append(cargos[i].Items, it)
		}
	}
	return rows.Err()
}

// insertCargoItems writes the per-place dimension rows for a cargo request.
func insertCargoItems(ctx context.Context, db Querier, cargoID uuid.UUID, items []models.CargoRequestItem) error {
	for i, it := range items {
		const q = `INSERT INTO cargo_request_items (id, cargo_request_id, position, length_m, width_m, height_m) VALUES ($1, $2, $3, $4, $5, $6)`
		if _, err := db.Exec(ctx, q, uuid.New(), cargoID, i, it.LengthM, it.WidthM, it.HeightM); err != nil {
			return err
		}
	}
	return nil
}

func scanCargoRequest(row pgx.Row) (*models.CargoRequest, error) {
	var c models.CargoRequest
	err := scanCargoRequestFields(row, &c)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CargoRequestRepository) Create(ctx context.Context, c *models.CargoRequest) error {
	const q = `
		INSERT INTO cargo_requests (
			id, client_id,
			origin_lat, origin_lng, origin_label, origin_source, origin_country, origin_labels,
			destination_lat, destination_lng, destination_label, destination_source, destination_country, destination_labels,
			volume_m3, weight_kg, category, description, status, created_at,
			packaging, places_count, stackable, adr_required
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
	`
	_, err := r.db.Exec(ctx, q,
		c.ID, c.ClientID,
		c.Origin.Lat, c.Origin.Lng, c.Origin.Label, c.Origin.Source, c.Origin.Country, marshalLabels(c.Origin.Labels),
		c.Destination.Lat, c.Destination.Lng, c.Destination.Label, c.Destination.Source, c.Destination.Country, marshalLabels(c.Destination.Labels),
		c.VolumeM3, c.WeightKg, c.Category, c.Description, c.Status, c.CreatedAt,
		c.Packaging, c.PlacesCount, c.Stackable, c.ADRRequired,
	)
	if err != nil {
		return err
	}
	return insertCargoItems(ctx, r.db, c.ID, c.Items)
}

func (r *CargoRequestRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.CargoRequestStatus) error {
	tag, err := r.db.Exec(ctx, `UPDATE cargo_requests SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *CargoRequestRepository) UpdateOpenOwned(ctx context.Context, c *models.CargoRequest) error {
	const q = `
		UPDATE cargo_requests SET
			origin_lat = $3, origin_lng = $4, origin_label = $5, origin_source = $6, origin_country = $7, origin_labels = $8,
			destination_lat = $9, destination_lng = $10, destination_label = $11, destination_source = $12, destination_country = $13, destination_labels = $14,
			volume_m3 = $15, weight_kg = $16, category = $17, description = $18,
			packaging = $19, places_count = $20, stackable = $21, adr_required = $22
		WHERE id = $1 AND client_id = $2 AND status = 'open'
	`
	tag, err := r.db.Exec(ctx, q,
		c.ID, c.ClientID,
		c.Origin.Lat, c.Origin.Lng, c.Origin.Label, c.Origin.Source, c.Origin.Country, marshalLabels(c.Origin.Labels),
		c.Destination.Lat, c.Destination.Lng, c.Destination.Label, c.Destination.Source, c.Destination.Country, marshalLabels(c.Destination.Labels),
		c.VolumeM3, c.WeightKg, c.Category, c.Description,
		c.Packaging, c.PlacesCount, c.Stackable, c.ADRRequired,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	// Replace the package rows to match the edited request.
	if _, err := r.db.Exec(ctx, `DELETE FROM cargo_request_items WHERE cargo_request_id = $1`, c.ID); err != nil {
		return err
	}
	return insertCargoItems(ctx, r.db, c.ID, c.Items)
}

func (r *CargoRequestRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.CargoRequest, error) {
	q := `SELECT ` + cargoRequestColumns + ` FROM cargo_requests WHERE id = $1`
	c, err := scanCargoRequest(r.db.QueryRow(ctx, q, id))
	if err != nil {
		return nil, err
	}
	single := []models.CargoRequest{*c}
	if err := attachCargoItems(ctx, r.db, single); err != nil {
		return nil, err
	}
	c.Items = single[0].Items
	return c, nil
}

// GetByIDForUpdate is GetByID with a row lock — serializes concurrent
// offer selection on the same cargo request. Only meaningful when the
// repository wraps a transaction.
func (r *CargoRequestRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.CargoRequest, error) {
	q := `SELECT ` + cargoRequestColumns + ` FROM cargo_requests WHERE id = $1 FOR UPDATE`
	return scanCargoRequest(r.db.QueryRow(ctx, q, id))
}

func (r *CargoRequestRepository) ListByClientID(ctx context.Context, clientID uuid.UUID) ([]models.CargoRequest, error) {
	q := `SELECT ` + cargoRequestColumns + ` FROM cargo_requests WHERE client_id = $1 ORDER BY created_at DESC`
	return queryCargoRequests(ctx, r.db, q, clientID)
}

// ActiveVolumeByOwnedRoute sums the caller's open/matched cargo requests for
// every saved direction. This is the platform part of a warehouse dispatch
// plan's "already collected" volume.
func (r *CargoRequestRepository) ActiveVolumeByOwnedRoute(ctx context.Context, userID uuid.UUID, cnKm, kzKm float64) (map[uuid.UUID]float64, error) {
	const q = `
		SELECT pr.id, COALESCE(SUM(cr.volume_m3), 0)::float8
		FROM participant_routes pr
		LEFT JOIN cargo_requests cr
		  ON cr.client_id = pr.user_id
		 AND cr.status IN ('open', 'matched')
		 AND haversine_km(pr.origin_lat, pr.origin_lng, cr.origin_lat, cr.origin_lng)
		     <= GREATEST(
		          CASE WHEN pr.origin_country = 'cn' THEN $2::float8 ELSE $3::float8 END,
		          CASE WHEN cr.origin_country = 'cn' THEN $2::float8 ELSE $3::float8 END)
		 AND haversine_km(pr.destination_lat, pr.destination_lng, cr.destination_lat, cr.destination_lng)
		     <= GREATEST(
		          CASE WHEN pr.destination_country = 'cn' THEN $2::float8 ELSE $3::float8 END,
		          CASE WHEN cr.destination_country = 'cn' THEN $2::float8 ELSE $3::float8 END)
		WHERE pr.user_id = $1
		GROUP BY pr.id
	`
	rows, err := r.db.Query(ctx, q, userID, cnKm, kzKm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make(map[uuid.UUID]float64)
	for rows.Next() {
		var routeID uuid.UUID
		var volumeM3 float64
		if err := rows.Scan(&routeID, &volumeM3); err != nil {
			return nil, err
		}
		items[routeID] = volumeM3
	}
	return items, rows.Err()
}

// ListOpenMatchingUserRoutes powers /cargo/available: open requests where
// the participant has at least one route whose origin AND destination are
// both within the per-country radius of the request's endpoints. The
// threshold for each compared pair is CASE country='cn' → cnKm, else kzKm
// (unknown/empty country falls into the kz bucket); if the two points of a
// pair are in different countries (border case), GREATEST takes the more
// generous of their radii. A participant with no routes gets an empty list
// by construction.
func (r *CargoRequestRepository) ListOpenMatchingUserRoutes(ctx context.Context, userID uuid.UUID, cnKm, kzKm float64) ([]models.CargoRequest, error) {
	q := `
		SELECT ` + cargoRequestColumns + `
		FROM cargo_requests cr
		WHERE cr.status = 'open' AND EXISTS (
			SELECT 1 FROM participant_routes pr
			WHERE pr.user_id = $1
			  -- Широтная полоса (sargable) до haversine: индекс из миграции
			  -- 000030 отсекает заведомо далёкие строки, точность даёт
			  -- haversine ниже.
			  AND cr.origin_lat BETWEEN pr.origin_lat - GREATEST($2::float8, $3::float8) / 110.0
			                        AND pr.origin_lat + GREATEST($2::float8, $3::float8) / 110.0
			  AND haversine_km(pr.origin_lat, pr.origin_lng, cr.origin_lat, cr.origin_lng)
			      <= GREATEST(
			           CASE WHEN pr.origin_country = 'cn' THEN $2::float8 ELSE $3::float8 END,
			           CASE WHEN cr.origin_country = 'cn' THEN $2::float8 ELSE $3::float8 END)
			  AND haversine_km(pr.destination_lat, pr.destination_lng, cr.destination_lat, cr.destination_lng)
			      <= GREATEST(
			           CASE WHEN pr.destination_country = 'cn' THEN $2::float8 ELSE $3::float8 END,
			           CASE WHEN cr.destination_country = 'cn' THEN $2::float8 ELSE $3::float8 END)
		)
		ORDER BY cr.created_at DESC
	`
	return queryCargoRequests(ctx, r.db, q, userID, cnKm, kzKm)
}

// ListOpenMatchingOwnerWarehouses returns open cargo requests that at least one
// of the owner's published, pickup-enabled warehouses can collect: cargo origin
// within pickup_radius of the warehouse address OR a pickup city, and — if the
// warehouse declares dispatch routes — the cargo destination near one of them.
func (r *CargoRequestRepository) ListOpenMatchingOwnerWarehouses(ctx context.Context, ownerID uuid.UUID, dispatchRadiusKm float64) ([]models.CargoRequest, error) {
	q := `
		SELECT ` + cargoRequestColumns + `
		FROM cargo_requests cr
		WHERE cr.status = 'open' AND EXISTS (
			SELECT 1 FROM warehouses w
			WHERE w.user_id = $1 AND w.status = 'published' AND w.pickup_enabled = true
			  AND (
				haversine_km(cr.origin_lat, cr.origin_lng, (w.address->>'lat')::float8, (w.address->>'lng')::float8) <= w.pickup_radius_km
				OR EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(NULLIF(w.pickup_cities, 'null'::jsonb), '[]'::jsonb)) pc
				           WHERE haversine_km(cr.origin_lat, cr.origin_lng, (pc->>'lat')::float8, (pc->>'lng')::float8) <= w.pickup_radius_km)
			  )
			  AND (
				jsonb_array_length(COALESCE(NULLIF(w.dispatch_routes, 'null'::jsonb), '[]'::jsonb)) = 0
				OR EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(NULLIF(w.dispatch_routes, 'null'::jsonb), '[]'::jsonb)) dr
				           WHERE haversine_km(cr.destination_lat, cr.destination_lng, (dr->'destination'->>'lat')::float8, (dr->'destination'->>'lng')::float8) <= $2::float8)
			  )
		)
		ORDER BY cr.created_at DESC
	`
	return queryCargoRequests(ctx, r.db, q, ownerID, dispatchRadiusKm)
}

func queryCargoRequests(ctx context.Context, db Querier, q string, args ...any) ([]models.CargoRequest, error) {
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.CargoRequest, 0)
	for rows.Next() {
		var c models.CargoRequest
		if err := scanCargoRequestFields(rows, &c); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachCargoItems(ctx, db, items); err != nil {
		return nil, err
	}
	return items, nil
}
