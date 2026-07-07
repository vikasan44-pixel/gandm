package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type OfferRepository struct {
	db Querier
}

func NewOfferRepository(db Querier) *OfferRepository {
	return &OfferRepository{db: db}
}

const offerColumns = `id, cargo_request_id, consolidated_request_id, participant_id, price, currency, conditions, warehouse_fill_percent, status, created_at`

func scanOfferFields(row pgx.Row, o *models.Offer) error {
	return row.Scan(
		&o.ID, &o.CargoRequestID, &o.ConsolidatedRequestID, &o.ParticipantID,
		&o.Price, &o.Currency, &o.Conditions, &o.WarehouseFillPercent, &o.Status, &o.CreatedAt,
	)
}

func (r *OfferRepository) Create(ctx context.Context, o *models.Offer) error {
	const q = `
		INSERT INTO offers (id, cargo_request_id, consolidated_request_id, participant_id, price, currency, conditions, warehouse_fill_percent, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, q,
		o.ID, o.CargoRequestID, o.ConsolidatedRequestID, o.ParticipantID,
		o.Price, o.Currency, o.Conditions, o.WarehouseFillPercent, o.Status, o.CreatedAt,
	)
	return err
}

func (r *OfferRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Offer, error) {
	q := `SELECT ` + offerColumns + ` FROM offers WHERE id = $1`
	var o models.Offer
	err := scanOfferFields(r.db.QueryRow(ctx, q, id), &o)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *OfferRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.OfferStatus) error {
	tag, err := r.db.Exec(ctx, `UPDATE offers SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByCargoRequestID orders by created_at ASC so the service layer can
// derive stable, anonymous "offer numbers" (1st submitted = #1) for the
// client-facing view.
func (r *OfferRepository) ListByCargoRequestID(ctx context.Context, cargoRequestID uuid.UUID) ([]models.Offer, error) {
	q := `SELECT ` + offerColumns + ` FROM offers WHERE cargo_request_id = $1 ORDER BY created_at ASC`
	return r.queryOffers(ctx, q, cargoRequestID)
}

// ListByConsolidatedRequestID — same ordering contract as above, for the
// shared consolidated competition.
func (r *OfferRepository) ListByConsolidatedRequestID(ctx context.Context, consolidatedID uuid.UUID) ([]models.Offer, error) {
	q := `SELECT ` + offerColumns + ` FROM offers WHERE consolidated_request_id = $1 ORDER BY created_at ASC`
	return r.queryOffers(ctx, q, consolidatedID)
}

func (r *OfferRepository) queryOffers(ctx context.Context, q string, args ...any) ([]models.Offer, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.Offer, 0)
	for rows.Next() {
		var o models.Offer
		if err := scanOfferFields(rows, &o); err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	return items, rows.Err()
}
