package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"gandm/internal/models"
)

var ErrAlreadyOffered = errors.New("this representative already submitted an offer for this consolidation")

type CustomsOfferRepository struct {
	db Querier
}

func NewCustomsOfferRepository(db Querier) *CustomsOfferRepository {
	return &CustomsOfferRepository{db: db}
}

const customsOfferColumns = `id, consolidated_request_id, customs_rep_id, price, currency, conditions, status, created_at`

func scanCustomsOffer(row pgx.Row) (*models.ConsolidatedCustomsOffer, error) {
	var o models.ConsolidatedCustomsOffer
	err := row.Scan(&o.ID, &o.ConsolidatedRequestID, &o.CustomsRepID, &o.Price, &o.Currency, &o.Conditions, &o.Status, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *CustomsOfferRepository) Create(ctx context.Context, o *models.ConsolidatedCustomsOffer) error {
	const q = `
		INSERT INTO consolidated_customs_offers (id, consolidated_request_id, customs_rep_id, price, currency, conditions, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (consolidated_request_id, customs_rep_id) DO UPDATE SET
			price = EXCLUDED.price,
			currency = EXCLUDED.currency,
			conditions = EXCLUDED.conditions,
			status = 'submitted'
		WHERE consolidated_customs_offers.status IN ('submitted', 'withdrawn')
	`
	tag, err := r.db.Exec(ctx, q, o.ID, o.ConsolidatedRequestID, o.CustomsRepID, o.Price, o.Currency, o.Conditions, o.Status, o.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyOffered
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAlreadyOffered
	}
	return nil
}

func (r *CustomsOfferRepository) UpdateSubmittedOwned(ctx context.Context, id, repID uuid.UUID, price float64, currency, conditions string) (*models.ConsolidatedCustomsOffer, error) {
	q := `UPDATE consolidated_customs_offers SET price = $3, currency = $4, conditions = $5
		WHERE id = $1 AND customs_rep_id = $2 AND status = 'submitted'
		RETURNING ` + customsOfferColumns
	o, err := scanCustomsOffer(r.db.QueryRow(ctx, q, id, repID, price, currency, conditions))
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *CustomsOfferRepository) WithdrawSubmittedOwned(ctx context.Context, id, repID uuid.UUID) (*models.ConsolidatedCustomsOffer, error) {
	q := `UPDATE consolidated_customs_offers SET status = 'withdrawn'
		WHERE id = $1 AND customs_rep_id = $2 AND status = 'submitted'
		RETURNING ` + customsOfferColumns
	o, err := scanCustomsOffer(r.db.QueryRow(ctx, q, id, repID))
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *CustomsOfferRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ConsolidatedCustomsOffer, error) {
	q := `SELECT ` + customsOfferColumns + ` FROM consolidated_customs_offers WHERE id = $1`
	return scanCustomsOffer(r.db.QueryRow(ctx, q, id))
}

// ListByConsolidatedID returns offers oldest-first — the stable order the
// anonymous offer numbers derive from.
func (r *CustomsOfferRepository) ListByConsolidatedID(ctx context.Context, consolidatedID uuid.UUID) ([]models.ConsolidatedCustomsOffer, error) {
	q := `SELECT ` + customsOfferColumns + ` FROM consolidated_customs_offers WHERE consolidated_request_id = $1 AND status != 'withdrawn' ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q, consolidatedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.ConsolidatedCustomsOffer, 0)
	for rows.Next() {
		var o models.ConsolidatedCustomsOffer
		if err := rows.Scan(&o.ID, &o.ConsolidatedRequestID, &o.CustomsRepID, &o.Price, &o.Currency, &o.Conditions, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	return items, rows.Err()
}

// ListByRepID returns the rep's own offers keyed by consolidation — used to
// mark competitions the rep already bid on.
func (r *CustomsOfferRepository) ListByRepID(ctx context.Context, repID uuid.UUID) (map[uuid.UUID]models.ConsolidatedCustomsOffer, error) {
	q := `SELECT ` + customsOfferColumns + ` FROM consolidated_customs_offers WHERE customs_rep_id = $1`
	rows, err := r.db.Query(ctx, q, repID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make(map[uuid.UUID]models.ConsolidatedCustomsOffer)
	for rows.Next() {
		var o models.ConsolidatedCustomsOffer
		if err := rows.Scan(&o.ID, &o.ConsolidatedRequestID, &o.CustomsRepID, &o.Price, &o.Currency, &o.Conditions, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		items[o.ConsolidatedRequestID] = o
	}
	return items, rows.Err()
}

// HasSelected reports whether the consolidation already has a chosen
// customs representative.
func (r *CustomsOfferRepository) HasSelected(ctx context.Context, consolidatedID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM consolidated_customs_offers WHERE consolidated_request_id = $1 AND status = 'selected')`
	var ok bool
	err := r.db.QueryRow(ctx, q, consolidatedID).Scan(&ok)
	return ok, err
}

// MarkSelected flips one offer to selected and every other submitted offer
// of the consolidation to rejected, in two statements — call inside a
// transaction.
func (r *CustomsOfferRepository) MarkSelected(ctx context.Context, consolidatedID, offerID uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE consolidated_customs_offers SET status = 'selected' WHERE id = $1 AND consolidated_request_id = $2`,
		offerID, consolidatedID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = r.db.Exec(ctx,
		`UPDATE consolidated_customs_offers SET status = 'rejected' WHERE consolidated_request_id = $1 AND id != $2 AND status = 'submitted'`,
		consolidatedID, offerID)
	return err
}
