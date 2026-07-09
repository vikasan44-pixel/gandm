package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"gandm/internal/models"
)

var ErrAlreadyBid = errors.New("this driver already bid on this competition")

type DriverCompetitionRepository struct {
	db Querier
}

func NewDriverCompetitionRepository(db Querier) *DriverCompetitionRepository {
	return &DriverCompetitionRepository{db: db}
}

const driverCompetitionColumns = `id, warehouse_id, route_id, volume_m3, dispatch_date, status, created_at`

func scanDriverCompetition(row pgx.Row) (*models.DriverCompetition, error) {
	var c models.DriverCompetition
	err := row.Scan(&c.ID, &c.WarehouseID, &c.RouteID, &c.VolumeM3, &c.DispatchDate, &c.Status, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *DriverCompetitionRepository) Create(ctx context.Context, c *models.DriverCompetition) error {
	const q = `
		INSERT INTO driver_competitions (id, warehouse_id, route_id, volume_m3, dispatch_date, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, q, c.ID, c.WarehouseID, c.RouteID, c.VolumeM3, c.DispatchDate, c.Status, c.CreatedAt)
	return err
}

func (r *DriverCompetitionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.DriverCompetition, error) {
	q := `SELECT ` + driverCompetitionColumns + ` FROM driver_competitions WHERE id = $1`
	return scanDriverCompetition(r.db.QueryRow(ctx, q, id))
}

// GetByIDForUpdate — row lock, serializes the select-winner path.
func (r *DriverCompetitionRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.DriverCompetition, error) {
	q := `SELECT ` + driverCompetitionColumns + ` FROM driver_competitions WHERE id = $1 FOR UPDATE`
	return scanDriverCompetition(r.db.QueryRow(ctx, q, id))
}

func (r *DriverCompetitionRepository) ListByWarehouseID(ctx context.Context, warehouseID uuid.UUID) ([]models.DriverCompetition, error) {
	q := `SELECT ` + driverCompetitionColumns + ` FROM driver_competitions WHERE warehouse_id = $1 ORDER BY created_at DESC`
	return r.query(ctx, q, warehouseID)
}

// ListOpen returns all open competitions — the driver-side feed. The
// warehouse identity stays server-side; the service exposes only direction
// labels and totals (ТЗ §11.4: «без названия склада»).
func (r *DriverCompetitionRepository) ListOpen(ctx context.Context) ([]models.DriverCompetition, error) {
	q := `SELECT ` + driverCompetitionColumns + ` FROM driver_competitions WHERE status = 'open' ORDER BY created_at DESC`
	return r.query(ctx, q)
}

// HasOpenForRoute prevents duplicate auto-announcements for one route.
func (r *DriverCompetitionRepository) HasOpenForRoute(ctx context.Context, routeID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM driver_competitions WHERE route_id = $1 AND status = 'open')`
	var ok bool
	err := r.db.QueryRow(ctx, q, routeID).Scan(&ok)
	return ok, err
}

func (r *DriverCompetitionRepository) Close(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `UPDATE driver_competitions SET status = 'closed' WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *DriverCompetitionRepository) query(ctx context.Context, q string, args ...any) ([]models.DriverCompetition, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.DriverCompetition, 0)
	for rows.Next() {
		var c models.DriverCompetition
		if err := rows.Scan(&c.ID, &c.WarehouseID, &c.RouteID, &c.VolumeM3, &c.DispatchDate, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// --- bids ---

const driverBidColumns = `id, competition_id, driver_id, price, currency, comment, status, created_at`

func (r *DriverCompetitionRepository) CreateBid(ctx context.Context, b *models.DriverCompetitionBid) error {
	const q = `
		INSERT INTO driver_competition_bids (id, competition_id, driver_id, price, currency, comment, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, q, b.ID, b.CompetitionID, b.DriverID, b.Price, b.Currency, b.Comment, b.Status, b.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAlreadyBid
		}
		return err
	}
	return nil
}

func (r *DriverCompetitionRepository) GetBidByID(ctx context.Context, id uuid.UUID) (*models.DriverCompetitionBid, error) {
	q := `SELECT ` + driverBidColumns + ` FROM driver_competition_bids WHERE id = $1`
	var b models.DriverCompetitionBid
	err := r.db.QueryRow(ctx, q, id).Scan(&b.ID, &b.CompetitionID, &b.DriverID, &b.Price, &b.Currency, &b.Comment, &b.Status, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// ListBidsByCompetitionID: oldest-first — stable anonymous numbering.
func (r *DriverCompetitionRepository) ListBidsByCompetitionID(ctx context.Context, competitionID uuid.UUID) ([]models.DriverCompetitionBid, error) {
	q := `SELECT ` + driverBidColumns + ` FROM driver_competition_bids WHERE competition_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q, competitionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.DriverCompetitionBid, 0)
	for rows.Next() {
		var b models.DriverCompetitionBid
		if err := rows.Scan(&b.ID, &b.CompetitionID, &b.DriverID, &b.Price, &b.Currency, &b.Comment, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

// ListBidsByDriverID keyed by competition — marks feeds the driver already
// bid on.
func (r *DriverCompetitionRepository) ListBidsByDriverID(ctx context.Context, driverID uuid.UUID) (map[uuid.UUID]models.DriverCompetitionBid, error) {
	q := `SELECT ` + driverBidColumns + ` FROM driver_competition_bids WHERE driver_id = $1`
	rows, err := r.db.Query(ctx, q, driverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make(map[uuid.UUID]models.DriverCompetitionBid)
	for rows.Next() {
		var b models.DriverCompetitionBid
		if err := rows.Scan(&b.ID, &b.CompetitionID, &b.DriverID, &b.Price, &b.Currency, &b.Comment, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		items[b.CompetitionID] = b
	}
	return items, rows.Err()
}

// MarkBidSelected flips the winner and rejects the rest — call inside a
// transaction.
func (r *DriverCompetitionRepository) MarkBidSelected(ctx context.Context, competitionID, bidID uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE driver_competition_bids SET status = 'selected' WHERE id = $1 AND competition_id = $2`,
		bidID, competitionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = r.db.Exec(ctx,
		`UPDATE driver_competition_bids SET status = 'rejected' WHERE competition_id = $1 AND id != $2 AND status = 'submitted'`,
		competitionID, bidID)
	return err
}
