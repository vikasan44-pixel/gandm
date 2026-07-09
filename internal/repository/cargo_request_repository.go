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

const cargoRequestColumns = `id, client_id, origin_lat, origin_lng, origin_label, origin_source, origin_country, destination_lat, destination_lng, destination_label, destination_source, destination_country, volume_m3, weight_kg, description, status, created_at`

func scanCargoRequestFields(row pgx.Row, c *models.CargoRequest) error {
	return row.Scan(
		&c.ID, &c.ClientID,
		&c.Origin.Lat, &c.Origin.Lng, &c.Origin.Label, &c.Origin.Source, &c.Origin.Country,
		&c.Destination.Lat, &c.Destination.Lng, &c.Destination.Label, &c.Destination.Source, &c.Destination.Country,
		&c.VolumeM3, &c.WeightKg, &c.Description, &c.Status, &c.CreatedAt,
	)
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
			origin_lat, origin_lng, origin_label, origin_source, origin_country,
			destination_lat, destination_lng, destination_label, destination_source, destination_country,
			volume_m3, weight_kg, description, status, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`
	_, err := r.db.Exec(ctx, q,
		c.ID, c.ClientID,
		c.Origin.Lat, c.Origin.Lng, c.Origin.Label, c.Origin.Source, c.Origin.Country,
		c.Destination.Lat, c.Destination.Lng, c.Destination.Label, c.Destination.Source, c.Destination.Country,
		c.VolumeM3, c.WeightKg, c.Description, c.Status, c.CreatedAt,
	)
	return err
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

func (r *CargoRequestRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.CargoRequest, error) {
	q := `SELECT ` + cargoRequestColumns + ` FROM cargo_requests WHERE id = $1`
	return scanCargoRequest(r.db.QueryRow(ctx, q, id))
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
	return items, rows.Err()
}
