package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type FillReportRepository struct {
	db Querier
}

func NewFillReportRepository(db Querier) *FillReportRepository {
	return &FillReportRepository{db: db}
}

const fillReportColumns = `id, user_id, expected_fill_percent, actual_fill_percent, photo_url, report_date, created_at`

func scanFillReport(row pgx.Row, fr *models.WarehouseFillReport) error {
	return row.Scan(&fr.ID, &fr.UserID, &fr.ExpectedFillPercent, &fr.ActualFillPercent, &fr.PhotoURL, &fr.ReportDate, &fr.CreatedAt)
}

func (r *FillReportRepository) Create(ctx context.Context, fr *models.WarehouseFillReport) error {
	const q = `
		INSERT INTO warehouse_fill_reports (id, user_id, expected_fill_percent, actual_fill_percent, photo_url, report_date, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, q, fr.ID, fr.UserID, fr.ExpectedFillPercent, fr.ActualFillPercent, fr.PhotoURL, fr.ReportDate, fr.CreatedAt)
	return err
}

func (r *FillReportRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.WarehouseFillReport, error) {
	q := `SELECT ` + fillReportColumns + ` FROM warehouse_fill_reports WHERE user_id = $1 ORDER BY report_date DESC, created_at DESC LIMIT 100`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.WarehouseFillReport, 0)
	for rows.Next() {
		var fr models.WarehouseFillReport
		if err := scanFillReport(rows, &fr); err != nil {
			return nil, err
		}
		items = append(items, fr)
	}
	return items, rows.Err()
}

func (r *FillReportRepository) LatestByUserID(ctx context.Context, userID uuid.UUID) (*models.WarehouseFillReport, error) {
	q := `SELECT ` + fillReportColumns + ` FROM warehouse_fill_reports WHERE user_id = $1 ORDER BY report_date DESC, created_at DESC LIMIT 1`
	var fr models.WarehouseFillReport
	err := scanFillReport(r.db.QueryRow(ctx, q, userID), &fr)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &fr, nil
}

// LatestForUsers bulk-fetches each user's newest report — feeds the
// anonymized offer lists without leaking who the warehouse is.
func (r *FillReportRepository) LatestForUsers(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]models.WarehouseFillReport, error) {
	out := make(map[uuid.UUID]models.WarehouseFillReport, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	q := `
		SELECT DISTINCT ON (user_id) ` + fillReportColumns + `
		FROM warehouse_fill_reports
		WHERE user_id = ANY($1)
		ORDER BY user_id, report_date DESC, created_at DESC
	`
	rows, err := r.db.Query(ctx, q, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var fr models.WarehouseFillReport
		if err := scanFillReport(rows, &fr); err != nil {
			return nil, err
		}
		out[fr.UserID] = fr
	}
	return out, rows.Err()
}
