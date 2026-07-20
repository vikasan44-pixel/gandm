package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type RefreshTokenRepository struct {
	db Querier
}

func NewRefreshTokenRepository(db Querier) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Insert(ctx context.Context, jti, subjectID uuid.UUID, subjectType string, issuedAt, expiresAt time.Time) error {
	const q = `
		INSERT INTO refresh_tokens (jti, subject_id, subject_type, issued_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, q, jti, subjectID, subjectType, issuedAt, expiresAt)
	return err
}

// GetForUpdate loads and row-locks the token so rotation (validate → revoke
// old → insert new) is atomic against a concurrent refresh with the same jti.
// Only meaningful when the repository wraps a transaction.
func (r *RefreshTokenRepository) GetForUpdate(ctx context.Context, jti uuid.UUID) (*models.RefreshToken, error) {
	const q = `
		SELECT jti, subject_id, subject_type, issued_at, expires_at, revoked_at, replaced_by
		FROM refresh_tokens WHERE jti = $1 FOR UPDATE
	`
	var t models.RefreshToken
	err := r.db.QueryRow(ctx, q, jti).Scan(
		&t.JTI, &t.SubjectID, &t.SubjectType, &t.IssuedAt, &t.ExpiresAt, &t.RevokedAt, &t.ReplacedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Revoke marks a single token revoked (no-op if already revoked), optionally
// recording the jti that replaced it during rotation.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, jti uuid.UUID, replacedBy *uuid.UUID, at time.Time) error {
	const q = `UPDATE refresh_tokens SET revoked_at = $2, replaced_by = $3 WHERE jti = $1 AND revoked_at IS NULL`
	_, err := r.db.Exec(ctx, q, jti, at, replacedBy)
	return err
}

// RevokeAllForSubject kills every active token of the subject — used on
// logout, account block, password change, and refresh-token reuse detection.
func (r *RefreshTokenRepository) RevokeAllForSubject(ctx context.Context, subjectType string, subjectID uuid.UUID, at time.Time) error {
	const q = `UPDATE refresh_tokens SET revoked_at = $3 WHERE subject_type = $1 AND subject_id = $2 AND revoked_at IS NULL`
	_, err := r.db.Exec(ctx, q, subjectType, subjectID, at)
	return err
}

// DeleteExpired removes old registry rows after a short audit/replay window.
// Without cleanup every successful refresh permanently grows the table.
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context, before time.Time) error {
	_, err := r.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < $1`, before)
	return err
}
