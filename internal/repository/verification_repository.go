package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type VerificationRepository struct {
	db Querier
}

func NewVerificationRepository(db Querier) *VerificationRepository {
	return &VerificationRepository{db: db}
}

const verificationColumns = `id, user_id, status, reject_reason, reviewed_by, reviewed_at, created_at`

func scanVerification(row pgx.Row) (*models.VerificationRequest, error) {
	var v models.VerificationRequest
	err := row.Scan(&v.ID, &v.UserID, &v.Status, &v.RejectReason, &v.ReviewedBy, &v.ReviewedAt, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VerificationRepository) Create(ctx context.Context, v *models.VerificationRequest) error {
	const q = `
		INSERT INTO verification_requests (id, user_id, status, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.db.Exec(ctx, q, v.ID, v.UserID, v.Status, v.CreatedAt)
	return err
}

func (r *VerificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.VerificationRequest, error) {
	q := `SELECT ` + verificationColumns + ` FROM verification_requests WHERE id = $1`
	return scanVerification(r.db.QueryRow(ctx, q, id))
}

// GetByIDForUpdate serializes admin decisions for the same request.
func (r *VerificationRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.VerificationRequest, error) {
	q := `SELECT ` + verificationColumns + ` FROM verification_requests WHERE id = $1 FOR UPDATE`
	return scanVerification(r.db.QueryRow(ctx, q, id))
}

// GetLatestByUserID returns the most recent verification request for a user.
// There is exactly one per user today (created at registration), but this
// keeps working unchanged once resubmission after rejection is added.
func (r *VerificationRepository) GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*models.VerificationRequest, error) {
	q := `SELECT ` + verificationColumns + ` FROM verification_requests WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`
	return scanVerification(r.db.QueryRow(ctx, q, userID))
}

// QueueItem is a verification request joined with the participant summary
// needed to render the admin queue without N+1 queries.
type QueueItem struct {
	VerificationID  uuid.UUID                 `json:"verification_id"`
	UserID          uuid.UUID                 `json:"user_id"`
	Email           string                    `json:"email"`
	CompanyName     string                    `json:"company_name"`
	ParticipantType models.ParticipantType    `json:"participant_type"`
	Status          models.VerificationStatus `json:"status"`
	CreatedAt       time.Time                 `json:"created_at"`
}

// ListQueue returns verification requests with the given status, oldest
// first — the queue's "waited longest floats to the top" rule.
func (r *VerificationRepository) ListQueue(ctx context.Context, status models.VerificationStatus) ([]QueueItem, error) {
	const q = `
		SELECT vr.id, vr.user_id, u.email, u.company_name, u.participant_type, vr.status, vr.created_at
		FROM verification_requests vr
		JOIN users u ON u.id = vr.user_id
		WHERE vr.status = $1
		ORDER BY vr.created_at ASC
	`
	rows, err := r.db.Query(ctx, q, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]QueueItem, 0)
	for rows.Next() {
		var it QueueItem
		if err := rows.Scan(&it.VerificationID, &it.UserID, &it.Email, &it.CompanyName, &it.ParticipantType, &it.Status, &it.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *VerificationRepository) CountByStatus(ctx context.Context, status models.VerificationStatus) (int, error) {
	const q = `SELECT count(*) FROM verification_requests WHERE status = $1`
	var n int
	err := r.db.QueryRow(ctx, q, status).Scan(&n)
	return n, err
}

func (r *VerificationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.VerificationStatus, rejectReason *string, reviewedBy uuid.UUID, reviewedAt time.Time) error {
	const q = `
		UPDATE verification_requests
		SET status = $2, reject_reason = $3, reviewed_by = $4, reviewed_at = $5
		WHERE id = $1 AND status = 'pending'
	`
	tag, err := r.db.Exec(ctx, q, id, status, rejectReason, reviewedBy, reviewedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
