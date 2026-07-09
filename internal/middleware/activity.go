package middleware

import (
	"net/http"
	"sync"
	"time"

	"gandm/internal/auth"
	"gandm/internal/repository"
)

// touchInterval bounds how often last_active_at is written per user. The
// stat consuming it (dashboard "visits") has calendar-day granularity, so
// anything under a few minutes is pure write amplification — especially
// with the frontend polling notifications every 30s.
const touchInterval = 5 * time.Minute

// TouchLastActive stamps users.last_active_at for authenticated requests,
// at most once per touchInterval per user (in-memory, resets on restart —
// an extra write after a restart is harmless). Must run after
// auth.RequireAuth so the user id is in context. Errors are ignored — a
// missed activity timestamp isn't worth failing the request over.
func TouchLastActive(userRepo *repository.UserRepository) func(http.Handler) http.Handler {
	var lastTouch sync.Map // uuid.UUID → time.Time
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, ok := auth.UserIDFromContext(r.Context()); ok {
				now := time.Now()
				prev, seen := lastTouch.Load(userID)
				if !seen || now.Sub(prev.(time.Time)) >= touchInterval {
					lastTouch.Store(userID, now)
					_ = userRepo.TouchLastActive(r.Context(), userID)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
