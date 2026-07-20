package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"gandm/internal/models"
)

var ErrRouteExists = errors.New("route already exists for this participant")

type ParticipantRouteRepository struct {
	db Querier
}

func NewParticipantRouteRepository(db Querier) *ParticipantRouteRepository {
	return &ParticipantRouteRepository{db: db}
}

const participantRouteColumns = `id, user_id, origin_lat, origin_lng, origin_label, origin_source, origin_country, origin_labels, destination_lat, destination_lng, destination_label, destination_source, destination_country, destination_labels, created_at`

func scanParticipantRouteFields(row pgx.Row, pr *models.ParticipantRoute) error {
	var originLabels, destinationLabels []byte
	err := row.Scan(
		&pr.ID, &pr.UserID,
		&pr.Origin.Lat, &pr.Origin.Lng, &pr.Origin.Label, &pr.Origin.Source, &pr.Origin.Country, &originLabels,
		&pr.Destination.Lat, &pr.Destination.Lng, &pr.Destination.Label, &pr.Destination.Source, &pr.Destination.Country, &destinationLabels,
		&pr.CreatedAt,
	)
	if err != nil {
		return err
	}
	pr.Origin.Labels = scanLabels(originLabels)
	pr.Destination.Labels = scanLabels(destinationLabels)
	return nil
}

func scanParticipantRoute(row pgx.Row) (*models.ParticipantRoute, error) {
	var pr models.ParticipantRoute
	err := scanParticipantRouteFields(row, &pr)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &pr, nil
}

func (r *ParticipantRouteRepository) Create(ctx context.Context, pr *models.ParticipantRoute) error {
	return r.create(ctx, pr, "manual")
}

func (r *ParticipantRouteRepository) CreateForWarehouse(ctx context.Context, pr *models.ParticipantRoute) error {
	return r.create(ctx, pr, "warehouse")
}

func (r *ParticipantRouteRepository) create(ctx context.Context, pr *models.ParticipantRoute, source string) error {
	const q = `
		INSERT INTO participant_routes (
			id, user_id,
			origin_lat, origin_lng, origin_label, origin_source, origin_country, origin_labels,
			destination_lat, destination_lng, destination_label, destination_source, destination_country, destination_labels,
			created_at, route_source
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	_, err := r.db.Exec(ctx, q,
		pr.ID, pr.UserID,
		pr.Origin.Lat, pr.Origin.Lng, pr.Origin.Label, pr.Origin.Source, pr.Origin.Country, marshalLabels(pr.Origin.Labels),
		pr.Destination.Lat, pr.Destination.Lng, pr.Destination.Label, pr.Destination.Source, pr.Destination.Country, marshalLabels(pr.Destination.Labels),
		pr.CreatedAt, source,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrRouteExists
		}
		return err
	}
	return nil
}

// ReplaceWarehouseLinks makes the relational route set match the warehouse
// snapshot and removes only orphaned warehouse-created routes. Manual routes
// are never deleted by a warehouse edit.
func (r *ParticipantRouteRepository) ReplaceWarehouseLinks(ctx context.Context, warehouseID uuid.UUID, routeIDs []uuid.UUID) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM warehouse_dispatch_route_links WHERE warehouse_id = $1`, warehouseID); err != nil {
		return err
	}
	for _, routeID := range routeIDs {
		if _, err := r.db.Exec(ctx, `INSERT INTO warehouse_dispatch_route_links (warehouse_id, route_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, warehouseID, routeID); err != nil {
			return err
		}
	}
	_, err := r.db.Exec(ctx, `DELETE FROM participant_routes route
		WHERE route.route_source = 'warehouse'
		  AND NOT EXISTS (SELECT 1 FROM warehouse_dispatch_route_links link WHERE link.route_id = route.id)
		  AND NOT EXISTS (SELECT 1 FROM warehouse_dispatch_thresholds threshold WHERE threshold.route_id = route.id)`)
	return err
}

func (r *ParticipantRouteRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ParticipantRoute, error) {
	q := `SELECT ` + participantRouteColumns + ` FROM participant_routes WHERE id = $1`
	return scanParticipantRoute(r.db.QueryRow(ctx, q, id))
}

func (r *ParticipantRouteRepository) GetByUserAndPoints(ctx context.Context, userID uuid.UUID, origin, destination models.GeoPoint) (*models.ParticipantRoute, error) {
	q := `SELECT ` + participantRouteColumns + ` FROM participant_routes
		WHERE user_id = $1 AND origin_lat = $2 AND origin_lng = $3
		  AND destination_lat = $4 AND destination_lng = $5`
	return scanParticipantRoute(r.db.QueryRow(ctx, q, userID, origin.Lat, origin.Lng, destination.Lat, destination.Lng))
}

func (r *ParticipantRouteRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.ParticipantRoute, error) {
	q := `SELECT ` + participantRouteColumns + ` FROM participant_routes WHERE user_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := make([]models.ParticipantRoute, 0)
	for rows.Next() {
		var pr models.ParticipantRoute
		if err := scanParticipantRouteFields(rows, &pr); err != nil {
			return nil, err
		}
		routes = append(routes, pr)
	}
	return routes, rows.Err()
}

func (r *ParticipantRouteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM participant_routes WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
