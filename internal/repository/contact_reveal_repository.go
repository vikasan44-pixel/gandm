package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"gandm/internal/models"
)

var ErrAlreadyRevealed = errors.New("contact already revealed for this cargo request")

type ContactRevealRepository struct {
	db Querier
}

func NewContactRevealRepository(db Querier) *ContactRevealRepository {
	return &ContactRevealRepository{db: db}
}

func (r *ContactRevealRepository) Create(ctx context.Context, cr *models.ContactReveal) error {
	const q = `
		INSERT INTO contact_reveals (id, client_id, participant_id, cargo_request_id, is_paid, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, q, cr.ID, cr.ClientID, cr.ParticipantID, cr.CargoRequestID, cr.IsPaid, cr.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyRevealed
		}
		return err
	}
	return nil
}

// CountByClientID is the reveal-limit counter: lifetime total per client,
// not per cargo request.
func (r *ContactRevealRepository) CountByClientID(ctx context.Context, clientID uuid.UUID) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM contact_reveals WHERE client_id = $1`, clientID).Scan(&n)
	return n, err
}
