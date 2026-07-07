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

const participantRouteColumns = `id, user_id, origin_lat, origin_lng, origin_label, origin_source, origin_country, destination_lat, destination_lng, destination_label, destination_source, destination_country, created_at`

func scanParticipantRouteFields(row pgx.Row, pr *models.ParticipantRoute) error {
	return row.Scan(
		&pr.ID, &pr.UserID,
		&pr.Origin.Lat, &pr.Origin.Lng, &pr.Origin.Label, &pr.Origin.Source, &pr.Origin.Country,
		&pr.Destination.Lat, &pr.Destination.Lng, &pr.Destination.Label, &pr.Destination.Source, &pr.Destination.Country,
		&pr.CreatedAt,
	)
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
	const q = `
		INSERT INTO participant_routes (
			id, user_id,
			origin_lat, origin_lng, origin_label, origin_source, origin_country,
			destination_lat, destination_lng, destination_label, destination_source, destination_country,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.db.Exec(ctx, q,
		pr.ID, pr.UserID,
		pr.Origin.Lat, pr.Origin.Lng, pr.Origin.Label, pr.Origin.Source, pr.Origin.Country,
		pr.Destination.Lat, pr.Destination.Lng, pr.Destination.Label, pr.Destination.Source, pr.Destination.Country,
		pr.CreatedAt,
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

func (r *ParticipantRouteRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ParticipantRoute, error) {
	q := `SELECT ` + participantRouteColumns + ` FROM participant_routes WHERE id = $1`
	return scanParticipantRoute(r.db.QueryRow(ctx, q, id))
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
