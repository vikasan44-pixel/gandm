package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"gandm/internal/models"
)

var ErrWarehouseAlreadyOffered = errors.New("this warehouse already offered on this cargo")

type WarehouseOfferRepository struct {
	db Querier
}

func NewWarehouseOfferRepository(db Querier) *WarehouseOfferRepository {
	return &WarehouseOfferRepository{db: db}
}

const warehouseOfferColumns = `id, cargo_request_id, consolidated_request_id, warehouse_id, warehouse_owner_id, price, currency, conditions, status, chat_id, created_at, updated_at`

func scanWarehouseOffer(row pgx.Row) (*models.WarehouseOffer, error) {
	var o models.WarehouseOffer
	err := row.Scan(&o.ID, &o.CargoRequestID, &o.ConsolidatedRequestID, &o.WarehouseID, &o.WarehouseOwnerID, &o.Price, &o.Currency, &o.Conditions, &o.Status, &o.ChatID, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *WarehouseOfferRepository) Create(ctx context.Context, o *models.WarehouseOffer) error {
	const q = `
		INSERT INTO warehouse_offers (id, cargo_request_id, consolidated_request_id, warehouse_id, warehouse_owner_id, price, currency, conditions, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(ctx, q, o.ID, o.CargoRequestID, o.ConsolidatedRequestID, o.WarehouseID, o.WarehouseOwnerID, o.Price, o.Currency, o.Conditions, o.Status, o.CreatedAt, o.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrWarehouseAlreadyOffered
		}
		return err
	}
	return nil
}

func (r *WarehouseOfferRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.WarehouseOffer, error) {
	return scanWarehouseOffer(r.db.QueryRow(ctx, `SELECT `+warehouseOfferColumns+` FROM warehouse_offers WHERE id = $1 FOR UPDATE`, id))
}

func (r *WarehouseOfferRepository) ListByCargo(ctx context.Context, cargoID uuid.UUID) ([]models.WarehouseOffer, error) {
	// Chronological, not price-sorted: offers can be in different currencies
	// (worldwide marketplace, no FX conversion), so a numeric price sort across
	// them would be misleading. The client compares amounts + currencies itself.
	rows, err := r.db.Query(ctx, `SELECT `+warehouseOfferColumns+` FROM warehouse_offers WHERE cargo_request_id = $1 ORDER BY created_at ASC`, cargoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.WarehouseOffer, 0)
	for rows.Next() {
		o, err := scanWarehouseOffer(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *o)
	}
	return items, rows.Err()
}

// ListSubmittedByOwner returns the owner's own submitted/selected offers (their
// "my bids" view).
func (r *WarehouseOfferRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.WarehouseOffer, error) {
	rows, err := r.db.Query(ctx, `SELECT `+warehouseOfferColumns+` FROM warehouse_offers WHERE warehouse_owner_id = $1 ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.WarehouseOffer, 0)
	for rows.Next() {
		o, err := scanWarehouseOffer(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *o)
	}
	return items, rows.Err()
}

// MarkSelected selects one offer and rejects the other submitted offers on the
// same cargo, attaching the shared chat to the selected one.
func (r *WarehouseOfferRepository) MarkSelected(ctx context.Context, cargoID, offerID, chatID uuid.UUID) error {
	if _, err := r.db.Exec(ctx, `UPDATE warehouse_offers SET status = 'rejected', updated_at = now() WHERE cargo_request_id = $1 AND id <> $2 AND status = 'submitted'`, cargoID, offerID); err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, `UPDATE warehouse_offers SET status = 'selected', chat_id = $2, updated_at = now() WHERE id = $1`, offerID, chatID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *WarehouseOfferRepository) ListByConsolidated(ctx context.Context, consolidatedID uuid.UUID) ([]models.WarehouseOffer, error) {
	// Chronological, not price-sorted — offers may be in different currencies.
	rows, err := r.db.Query(ctx, `SELECT `+warehouseOfferColumns+` FROM warehouse_offers WHERE consolidated_request_id = $1 ORDER BY created_at ASC`, consolidatedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.WarehouseOffer, 0)
	for rows.Next() {
		o, err := scanWarehouseOffer(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *o)
	}
	return items, rows.Err()
}

// GetForConsolidated returns this warehouse's offer on a consolidation, or
// ErrNotFound. Used to decide create-vs-requote without triggering a unique
// violation (which would abort the surrounding transaction).
func (r *WarehouseOfferRepository) GetForConsolidated(ctx context.Context, consolidatedID, warehouseID uuid.UUID) (*models.WarehouseOffer, error) {
	return scanWarehouseOffer(r.db.QueryRow(ctx, `SELECT `+warehouseOfferColumns+` FROM warehouse_offers WHERE consolidated_request_id = $1 AND warehouse_id = $2`, consolidatedID, warehouseID))
}

func (r *WarehouseOfferRepository) HasSelectedForConsolidated(ctx context.Context, consolidatedID uuid.UUID) (bool, error) {
	var selected bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM warehouse_offers WHERE consolidated_request_id = $1 AND status = 'selected'
	)`, consolidatedID).Scan(&selected)
	return selected, err
}

// UpdateForConsolidated re-quotes an existing (not-yet-selected) warehouse
// offer and keeps it submitted — used after a late join changed the volume.
func (r *WarehouseOfferRepository) UpdateForConsolidated(ctx context.Context, consolidatedID, warehouseID uuid.UUID, price float64, currency, conditions string) error {
	const q = `UPDATE warehouse_offers SET price = $3, currency = $4, conditions = $5, status = 'submitted', updated_at = now()
		WHERE consolidated_request_id = $1 AND warehouse_id = $2 AND status <> 'selected'`
	tag, err := r.db.Exec(ctx, q, consolidatedID, warehouseID, price, currency, conditions)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListWarehouseIDsByConsolidated returns the warehouses that offered on a
// consolidation — to notify them to re-quote after a late join.
func (r *WarehouseOfferRepository) ListWarehouseIDsByConsolidated(ctx context.Context, consolidatedID uuid.UUID) ([]struct{ WarehouseID, OwnerID uuid.UUID }, error) {
	rows, err := r.db.Query(ctx, `SELECT warehouse_id, warehouse_owner_id FROM warehouse_offers WHERE consolidated_request_id = $1`, consolidatedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]struct{ WarehouseID, OwnerID uuid.UUID }, 0)
	for rows.Next() {
		var w struct{ WarehouseID, OwnerID uuid.UUID }
		if err := rows.Scan(&w.WarehouseID, &w.OwnerID); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// MarkSelectedForConsolidated selects one offer and rejects the others on the
// same consolidated request, attaching the shared chat.
func (r *WarehouseOfferRepository) MarkSelectedForConsolidated(ctx context.Context, consolidatedID, offerID, chatID uuid.UUID) error {
	if _, err := r.db.Exec(ctx, `UPDATE warehouse_offers SET status = 'rejected', updated_at = now() WHERE consolidated_request_id = $1 AND id <> $2 AND status = 'submitted'`, consolidatedID, offerID); err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, `UPDATE warehouse_offers SET status = 'selected', chat_id = $3, updated_at = now()
		WHERE id = $1 AND consolidated_request_id = $2 AND status = 'submitted'`, offerID, consolidatedID, chatID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
