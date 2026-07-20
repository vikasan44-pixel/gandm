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

// CreateOrUpdateSubmitted makes repeat submissions idempotent from the
// participant's point of view: while an active response exists, submitting
// the form again edits that response instead of creating a duplicate row.
// A response submitted after a withdrawal becomes a new history entry.
func (r *OfferRepository) CreateOrUpdateSubmitted(ctx context.Context, o *models.Offer) (*models.Offer, error) {
	conflictTarget := `(cargo_request_id, participant_id) WHERE cargo_request_id IS NOT NULL AND status = 'submitted'`
	if o.ConsolidatedRequestID != nil {
		conflictTarget = `(consolidated_request_id, participant_id) WHERE consolidated_request_id IS NOT NULL AND status = 'submitted'`
	}
	q := `
		INSERT INTO offers (id, cargo_request_id, consolidated_request_id, participant_id, price, currency, conditions, warehouse_fill_percent, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'submitted', $9)
		ON CONFLICT ` + conflictTarget + ` DO UPDATE SET
			price = EXCLUDED.price,
			currency = EXCLUDED.currency,
			conditions = EXCLUDED.conditions,
			warehouse_fill_percent = EXCLUDED.warehouse_fill_percent
		RETURNING ` + offerColumns
	var saved models.Offer
	err := scanOfferFields(r.db.QueryRow(ctx, q,
		o.ID, o.CargoRequestID, o.ConsolidatedRequestID, o.ParticipantID,
		o.Price, o.Currency, o.Conditions, o.WarehouseFillPercent, o.CreatedAt,
	), &saved)
	if err != nil {
		return nil, err
	}
	return &saved, nil
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

func (r *OfferRepository) UpdateSubmittedOwned(ctx context.Context, id, participantID uuid.UUID, price float64, currency, conditions string, fillPercent *float64) (*models.Offer, error) {
	q := `UPDATE offers
		SET price = $3, currency = $4, conditions = $5, warehouse_fill_percent = $6
		WHERE id = $1 AND participant_id = $2 AND status = 'submitted'
		RETURNING ` + offerColumns
	var o models.Offer
	err := scanOfferFields(r.db.QueryRow(ctx, q, id, participantID, price, currency, conditions, fillPercent), &o)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *OfferRepository) WithdrawSubmittedOwned(ctx context.Context, id, participantID uuid.UUID) (*models.Offer, error) {
	q := `UPDATE offers SET status = 'withdrawn'
		WHERE id = $1 AND participant_id = $2 AND status = 'submitted'
		RETURNING ` + offerColumns
	var o models.Offer
	err := scanOfferFields(r.db.QueryRow(ctx, q, id, participantID), &o)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *OfferRepository) RejectSubmittedForCargo(ctx context.Context, cargoRequestID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE offers SET status = 'rejected' WHERE cargo_request_id = $1 AND status = 'submitted'`, cargoRequestID)
	return err
}

func (r *OfferRepository) MarkSelectedForCargo(ctx context.Context, cargoRequestID, offerID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE offers SET status = 'selected' WHERE id = $1 AND cargo_request_id = $2 AND status = 'submitted'`, offerID, cargoRequestID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = r.db.Exec(ctx, `UPDATE offers SET status = 'rejected' WHERE cargo_request_id = $1 AND id != $2 AND status = 'submitted'`, cargoRequestID, offerID)
	return err
}

func (r *OfferRepository) MarkSelectedForConsolidated(ctx context.Context, consolidatedID, offerID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE offers SET status = 'selected' WHERE id = $1 AND consolidated_request_id = $2 AND status = 'submitted'`, offerID, consolidatedID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = r.db.Exec(ctx, `UPDATE offers SET status = 'rejected' WHERE consolidated_request_id = $1 AND id != $2 AND status = 'submitted'`, consolidatedID, offerID)
	return err
}

// ListByCargoRequestID orders by created_at ASC so the service layer can
// derive stable, anonymous "offer numbers" (1st submitted = #1) for the
// client-facing view.
func (r *OfferRepository) ListByCargoRequestID(ctx context.Context, cargoRequestID uuid.UUID) ([]models.Offer, error) {
	q := `SELECT ` + offerColumns + ` FROM offers WHERE cargo_request_id = $1 AND status != 'withdrawn' ORDER BY created_at ASC`
	return r.queryOffers(ctx, q, cargoRequestID)
}

// ListByConsolidatedRequestID — same ordering contract as above, for the
// shared consolidated competition.
func (r *OfferRepository) ListByConsolidatedRequestID(ctx context.Context, consolidatedID uuid.UUID) ([]models.Offer, error) {
	q := `SELECT ` + offerColumns + ` FROM offers WHERE consolidated_request_id = $1 AND status != 'withdrawn' ORDER BY created_at ASC`
	return r.queryOffers(ctx, q, consolidatedID)
}

// ListByParticipantID returns the participant's complete response history
// across single and consolidated cargo competitions.
func (r *OfferRepository) ListByParticipantID(ctx context.Context, participantID uuid.UUID) ([]models.Offer, error) {
	q := `SELECT ` + offerColumns + ` FROM offers WHERE participant_id = $1 ORDER BY created_at DESC`
	return r.queryOffers(ctx, q, participantID)
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
