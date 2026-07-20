package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"gandm/internal/auth"
	"gandm/internal/models"
	"gandm/internal/repository"
)

// ErrRefreshAlreadyRotated means another legitimate request rotated the same
// refresh token moments ago. The caller may retry with the replacement cookie;
// unlike a replay outside this short overlap window, this must not revoke the
// whole session family.
var ErrRefreshAlreadyRotated = errors.New("refresh token was already rotated")

const refreshRotationOverlapGrace = 5 * time.Second

func isRecentRefreshRotation(row *models.RefreshToken, now time.Time) bool {
	if row.RevokedAt == nil || row.ReplacedBy == nil {
		return false
	}
	age := now.Sub(*row.RevokedAt)
	return age >= 0 && age <= refreshRotationOverlapGrace
}

// IssuedSession is the result of a login/register/refresh: a short-lived access
// token (returned to the client for the Authorization header) plus a rotating
// refresh token that the handler stores in an httpOnly cookie, with the cookie
// expiry to match.
type IssuedSession struct {
	AccessToken    string
	RefreshToken   string
	RefreshExpires time.Time
}

// issueSession mints tokens for an already-created active session and records
// the refresh jti. The caller owns the surrounding transaction.
func issueSession(ctx context.Context, db repository.Querier, tokens *auth.Manager, subjectID uuid.UUID, subjectType string, sessionID uuid.UUID) (IssuedSession, error) {
	access, err := tokens.IssueAccessToken(subjectID, subjectType, sessionID)
	if err != nil {
		return IssuedSession{}, err
	}
	jti := uuid.New()
	refresh, expiresAt, err := tokens.IssueRefreshToken(subjectID, subjectType, jti, sessionID)
	if err != nil {
		return IssuedSession{}, err
	}
	rtRepo := repository.NewRefreshTokenRepository(db)
	// Keep expired rows briefly for incident/replay analysis, then prune them.
	if err := rtRepo.DeleteExpired(ctx, time.Now().Add(-24*time.Hour)); err != nil {
		return IssuedSession{}, err
	}
	if err := rtRepo.Insert(ctx, jti, subjectID, subjectType, time.Now(), expiresAt); err != nil {
		return IssuedSession{}, err
	}
	return IssuedSession{AccessToken: access, RefreshToken: refresh, RefreshExpires: expiresAt}, nil
}

// startSingleSession replaces any previous login for the account and issues
// tokens for the replacement atomically. Concurrent logins serialize on the
// active_sessions primary key; the last committed login is the only valid one.
func startSingleSession(ctx context.Context, db *pgxpool.Pool, tokens *auth.Manager, subjectID uuid.UUID, subjectType string) (IssuedSession, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return IssuedSession{}, err
	}
	defer tx.Rollback(ctx)

	sess, err := startSingleSessionInTransaction(ctx, tx, tokens, subjectID, subjectType)
	if err != nil {
		return IssuedSession{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return IssuedSession{}, err
	}
	return sess, nil
}

// startSingleSessionInTransaction is also used by registration so the user,
// verification request, active session and first refresh token commit as one
// unit. The caller owns the transaction.
func startSingleSessionInTransaction(ctx context.Context, q repository.Querier, tokens *auth.Manager, subjectID uuid.UUID, subjectType string) (IssuedSession, error) {
	sessionID := uuid.New()
	if err := repository.NewActiveSessionRepository(q).Replace(ctx, subjectType, subjectID, sessionID); err != nil {
		return IssuedSession{}, err
	}
	rtRepo := repository.NewRefreshTokenRepository(q)
	if err := rtRepo.RevokeAllForSubject(ctx, subjectType, subjectID, time.Now()); err != nil {
		return IssuedSession{}, err
	}
	return issueSession(ctx, q, tokens, subjectID, subjectType, sessionID)
}

// rotateSession validates a presented refresh token and rotates it: the old jti
// is revoked and a brand-new pair is issued, all under a row lock so concurrent
// refreshes of the same token can't both succeed. `eligible` is called with the
// validated subject before issuing, so blocked/deleted subjects are refused
// without minting anything. On reuse of an already-revoked jti (a replay of a
// stolen token), the entire subject's session family is revoked and the call
// fails.
func rotateSession(
	ctx context.Context,
	db *pgxpool.Pool,
	tokens *auth.Manager,
	refreshToken, subjectType string,
	eligible func(ctx context.Context, subjectID uuid.UUID) error,
) (uuid.UUID, IssuedSession, error) {
	subjectID, jti, sessionID, err := tokens.ParseRefreshTokenDetailed(refreshToken, subjectType)
	if err != nil {
		return uuid.Nil, IssuedSession{}, ErrInvalidCredentials
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return uuid.Nil, IssuedSession{}, err
	}
	defer tx.Rollback(ctx)

	activeRepo := repository.NewActiveSessionRepository(tx)
	active, err := activeRepo.IsActiveForUpdate(ctx, subjectType, subjectID, sessionID)
	if err != nil {
		return uuid.Nil, IssuedSession{}, err
	}
	if !active {
		// A login on another device replaced this session. Never let the old
		// device's refresh attempt revoke the newer device's token family.
		return uuid.Nil, IssuedSession{}, ErrInvalidCredentials
	}

	rtRepo := repository.NewRefreshTokenRepository(tx)
	row, err := rtRepo.GetForUpdate(ctx, jti)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return uuid.Nil, IssuedSession{}, ErrInvalidCredentials
		}
		return uuid.Nil, IssuedSession{}, err
	}

	// Reuse detection: a revoked token being presented again means it was
	// replayed after rotation (its legitimate holder already swapped it, or it
	// was stolen). Revoke the whole family and refuse — the attacker and the
	// victim both lose the session, which is the safe outcome.
	if row.RevokedAt != nil {
		// Two tabs can submit the same cookie before the first rotation response
		// updates the browser's shared cookie jar. Treat that very short overlap
		// as a retryable race, not token theft. A later replay still revokes every
		// active token for the subject below.
		if isRecentRefreshRotation(row, time.Now()) {
			return uuid.Nil, IssuedSession{}, ErrRefreshAlreadyRotated
		}
		if err := activeRepo.DeleteIfMatches(ctx, subjectType, subjectID, sessionID); err != nil {
			return uuid.Nil, IssuedSession{}, err
		}
		if err := rtRepo.RevokeAllForSubject(ctx, subjectType, subjectID, time.Now()); err != nil {
			return uuid.Nil, IssuedSession{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return uuid.Nil, IssuedSession{}, err
		}
		return uuid.Nil, IssuedSession{}, ErrInvalidCredentials
	}
	if row.SubjectID != subjectID || row.SubjectType != subjectType {
		return uuid.Nil, IssuedSession{}, ErrInvalidCredentials
	}
	if time.Now().After(row.ExpiresAt) {
		return uuid.Nil, IssuedSession{}, ErrInvalidCredentials
	}

	if eligible != nil {
		if err := eligible(ctx, subjectID); err != nil {
			return uuid.Nil, IssuedSession{}, err
		}
	}

	access, err := tokens.IssueAccessToken(subjectID, subjectType, sessionID)
	if err != nil {
		return uuid.Nil, IssuedSession{}, err
	}
	newJTI := uuid.New()
	refresh, expiresAt, err := tokens.IssueRefreshToken(subjectID, subjectType, newJTI, sessionID)
	if err != nil {
		return uuid.Nil, IssuedSession{}, err
	}
	if err := rtRepo.Insert(ctx, newJTI, subjectID, subjectType, time.Now(), expiresAt); err != nil {
		return uuid.Nil, IssuedSession{}, err
	}
	if err := rtRepo.Revoke(ctx, jti, &newJTI, time.Now()); err != nil {
		return uuid.Nil, IssuedSession{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, IssuedSession{}, err
	}
	return subjectID, IssuedSession{AccessToken: access, RefreshToken: refresh, RefreshExpires: expiresAt}, nil
}

// revokeSessionByToken revokes the single refresh token presented (logout of
// the current session). A malformed/expired token is treated as already
// logged out — no error.
func revokeSessionByToken(ctx context.Context, db *pgxpool.Pool, tokens *auth.Manager, refreshToken, subjectType string) error {
	subjectID, jti, sessionID, err := tokens.ParseRefreshTokenDetailed(refreshToken, subjectType)
	if err != nil {
		return nil
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	activeRepo := repository.NewActiveSessionRepository(tx)
	if err := activeRepo.DeleteIfMatches(ctx, subjectType, subjectID, sessionID); err != nil {
		return err
	}
	rtRepo := repository.NewRefreshTokenRepository(tx)
	if err := rtRepo.Revoke(ctx, jti, nil, time.Now()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
