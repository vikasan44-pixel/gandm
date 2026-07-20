package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ActiveSessionRepository stores the one session currently allowed for each
// participant/admin account. It is intentionally polymorphic, like the refresh
// token registry, because users and admins live in separate tables.
type ActiveSessionRepository struct {
	db Querier
}

func NewActiveSessionRepository(db Querier) *ActiveSessionRepository {
	return &ActiveSessionRepository{db: db}
}

// Replace atomically makes sessionID the account's only active session.
func (r *ActiveSessionRepository) Replace(ctx context.Context, subjectType string, subjectID, sessionID uuid.UUID) error {
	const q = `
		INSERT INTO active_sessions (subject_type, subject_id, session_id, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (subject_type, subject_id) DO UPDATE
		SET session_id = EXCLUDED.session_id, updated_at = now()
	`
	_, err := r.db.Exec(ctx, q, subjectType, subjectID, sessionID)
	return err
}

func (r *ActiveSessionRepository) IsActive(ctx context.Context, subjectType string, subjectID, sessionID uuid.UUID) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1 FROM active_sessions
			WHERE subject_type = $1 AND subject_id = $2 AND session_id = $3
		)
	`
	var active bool
	err := r.db.QueryRow(ctx, q, subjectType, subjectID, sessionID).Scan(&active)
	return active, err
}

// IsActiveForUpdate also locks the account's active-session row. Login,
// refresh and logout all take this row first, giving them one consistent order
// and making their result deterministic under concurrent requests.
func (r *ActiveSessionRepository) IsActiveForUpdate(ctx context.Context, subjectType string, subjectID, sessionID uuid.UUID) (bool, error) {
	const q = `
		SELECT session_id FROM active_sessions
		WHERE subject_type = $1 AND subject_id = $2
		FOR UPDATE
	`
	var activeSessionID uuid.UUID
	err := r.db.QueryRow(ctx, q, subjectType, subjectID).Scan(&activeSessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return activeSessionID == sessionID, nil
}

// DeleteIfMatches logs out only the session presented by the caller. A late
// logout from an older device can never erase a newer device's session.
func (r *ActiveSessionRepository) DeleteIfMatches(ctx context.Context, subjectType string, subjectID, sessionID uuid.UUID) error {
	const q = `
		DELETE FROM active_sessions
		WHERE subject_type = $1 AND subject_id = $2 AND session_id = $3
	`
	_, err := r.db.Exec(ctx, q, subjectType, subjectID, sessionID)
	return err
}
