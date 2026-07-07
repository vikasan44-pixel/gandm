package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type ConsolidationRepository struct {
	db Querier
}

func NewConsolidationRepository(db Querier) *ConsolidationRepository {
	return &ConsolidationRepository{db: db}
}

const suggestionColumns = `id, cargo_request_a, cargo_request_b, direction_label, status, created_at`

func scanSuggestion(row pgx.Row) (*models.ConsolidationSuggestion, error) {
	var s models.ConsolidationSuggestion
	err := row.Scan(&s.ID, &s.CargoRequestA, &s.CargoRequestB, &s.DirectionLabel, &s.Status, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ConsolidationRepository) CreateSuggestion(ctx context.Context, s *models.ConsolidationSuggestion) error {
	const q = `
		INSERT INTO consolidation_suggestions (id, cargo_request_a, cargo_request_b, direction_label, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, q, s.ID, s.CargoRequestA, s.CargoRequestB, s.DirectionLabel, s.Status, s.CreatedAt)
	return err
}

func (r *ConsolidationRepository) GetSuggestionByID(ctx context.Context, id uuid.UUID) (*models.ConsolidationSuggestion, error) {
	q := `SELECT ` + suggestionColumns + ` FROM consolidation_suggestions WHERE id = $1`
	return scanSuggestion(r.db.QueryRow(ctx, q, id))
}

// GetActiveSuggestionByCargoID returns the pending suggestion (not yet
// resolved either way) that involves the given cargo request, if any.
func (r *ConsolidationRepository) GetActiveSuggestionByCargoID(ctx context.Context, cargoID uuid.UUID) (*models.ConsolidationSuggestion, error) {
	q := `
		SELECT ` + suggestionColumns + `
		FROM consolidation_suggestions
		WHERE (cargo_request_a = $1 OR cargo_request_b = $1)
		  AND status IN ('suggested', 'a_agreed', 'b_agreed')
		ORDER BY created_at DESC
		LIMIT 1
	`
	return scanSuggestion(r.db.QueryRow(ctx, q, cargoID))
}

// ExistsSuggestionForPair reports whether ANY suggestion (including a
// declined one) already exists for this pair, in either order. A pair that
// was declined once must not be re-suggested on every new cargo submission.
func (r *ConsolidationRepository) ExistsSuggestionForPair(ctx context.Context, a, b uuid.UUID) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM consolidation_suggestions
			WHERE (cargo_request_a = $1 AND cargo_request_b = $2)
			   OR (cargo_request_a = $2 AND cargo_request_b = $1)
		)
	`
	var exists bool
	err := r.db.QueryRow(ctx, q, a, b).Scan(&exists)
	return exists, err
}

func (r *ConsolidationRepository) UpdateSuggestionStatus(ctx context.Context, id uuid.UUID, status models.ConsolidationStatus) error {
	tag, err := r.db.Exec(ctx, `UPDATE consolidation_suggestions SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListOpenCargoWithoutActiveSuggestion returns the matching-service
// candidate pool: open, non-consolidated cargo requests not already tied up
// in a pending suggestion.
func (r *ConsolidationRepository) ListOpenCargoWithoutActiveSuggestion(ctx context.Context) ([]models.CargoRequest, error) {
	q := `
		SELECT ` + cargoRequestColumns + `
		FROM cargo_requests
		WHERE status = 'open' AND NOT EXISTS (
			SELECT 1 FROM consolidation_suggestions cs
			WHERE (cs.cargo_request_a = cargo_requests.id OR cs.cargo_request_b = cargo_requests.id)
			  AND cs.status IN ('suggested', 'a_agreed', 'b_agreed', 'both_agreed')
		)
		ORDER BY created_at ASC
	`
	return queryCargoRequests(ctx, r.db, q)
}

const consolidatedColumns = `id, origin_lat, origin_lng, origin_label, origin_country, destination_lat, destination_lng, destination_label, destination_country, total_volume_m3, total_weight_kg, member_request_ids, status, created_at`

func scanConsolidatedFields(row pgx.Row, c *models.ConsolidatedRequest) error {
	var memberIDs []byte
	err := row.Scan(
		&c.ID,
		&c.Origin.Lat, &c.Origin.Lng, &c.Origin.Label, &c.Origin.Country,
		&c.Destination.Lat, &c.Destination.Lng, &c.Destination.Label, &c.Destination.Country,
		&c.TotalVolumeM3, &c.TotalWeightKg, &memberIDs, &c.Status, &c.CreatedAt,
	)
	if err != nil {
		return err
	}
	return json.Unmarshal(memberIDs, &c.MemberRequestIDs)
}

func (r *ConsolidationRepository) CreateConsolidated(ctx context.Context, c *models.ConsolidatedRequest) error {
	memberIDs, err := json.Marshal(c.MemberRequestIDs)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO consolidated_requests (
			id, origin_lat, origin_lng, origin_label, origin_country,
			destination_lat, destination_lng, destination_label, destination_country,
			total_volume_m3, total_weight_kg, member_request_ids, status, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err = r.db.Exec(ctx, q,
		c.ID, c.Origin.Lat, c.Origin.Lng, c.Origin.Label, c.Origin.Country,
		c.Destination.Lat, c.Destination.Lng, c.Destination.Label, c.Destination.Country,
		c.TotalVolumeM3, c.TotalWeightKg, memberIDs, c.Status, c.CreatedAt,
	)
	return err
}

func (r *ConsolidationRepository) GetConsolidatedByID(ctx context.Context, id uuid.UUID) (*models.ConsolidatedRequest, error) {
	q := `SELECT ` + consolidatedColumns + ` FROM consolidated_requests WHERE id = $1`
	var c models.ConsolidatedRequest
	err := scanConsolidatedFields(r.db.QueryRow(ctx, q, id), &c)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListConsolidatedForClient returns consolidated requests where one of the
// member cargo requests belongs to the client (jsonb containment on the
// member id).
func (r *ConsolidationRepository) ListConsolidatedForClient(ctx context.Context, clientID uuid.UUID) ([]models.ConsolidatedRequest, error) {
	q := `
		SELECT ` + consolidatedColumns + `
		FROM consolidated_requests cr
		WHERE EXISTS (
			SELECT 1 FROM cargo_requests c
			WHERE c.client_id = $1 AND cr.member_request_ids @> to_jsonb(c.id::text)
		)
		ORDER BY cr.created_at DESC
	`
	return r.queryConsolidated(ctx, q, clientID)
}

// IsConsolidatedMember reports whether the client owns one of the member
// cargo requests — the access check for viewing the shared competition.
func (r *ConsolidationRepository) IsConsolidatedMember(ctx context.Context, clientID, consolidatedID uuid.UUID) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM consolidated_requests cr
			JOIN cargo_requests c ON c.client_id = $1
			WHERE cr.id = $2 AND cr.member_request_ids @> to_jsonb(c.id::text)
		)
	`
	var ok bool
	err := r.db.QueryRow(ctx, q, clientID, consolidatedID).Scan(&ok)
	return ok, err
}

// ListOpenConsolidatedMatchingUserRoutes mirrors the single-cargo available
// query: open consolidated requests whose endpoints fall within the
// per-country radius of one of the participant's routes.
func (r *ConsolidationRepository) ListOpenConsolidatedMatchingUserRoutes(ctx context.Context, userID uuid.UUID, cnKm, kzKm float64) ([]models.ConsolidatedRequest, error) {
	q := `
		SELECT ` + consolidatedColumns + `
		FROM consolidated_requests cr
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
	return r.queryConsolidated(ctx, q, userID, cnKm, kzKm)
}

func (r *ConsolidationRepository) queryConsolidated(ctx context.Context, q string, args ...any) ([]models.ConsolidatedRequest, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.ConsolidatedRequest, 0)
	for rows.Next() {
		var c models.ConsolidatedRequest
		if err := scanConsolidatedFields(rows, &c); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
