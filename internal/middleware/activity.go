package middleware

import (
	"net/http"
	"sync"
	"sync/atomic"
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
	var entries atomic.Int64

	// sweep удаляет протухшие записи, когда карта разрастается — иначе за
	// месяцы работы она копила бы по записи на каждого когда-либо
	// заходившего пользователя (медленная утечка памяти).
	sweep := func(now time.Time) {
		lastTouch.Range(func(key, value any) bool {
			if now.Sub(value.(time.Time)) >= touchInterval {
				lastTouch.Delete(key)
				entries.Add(-1)
			}
			return true
		})
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, ok := auth.UserIDFromContext(r.Context()); ok {
				now := time.Now()
				prev, seen := lastTouch.Load(userID)
				if !seen || now.Sub(prev.(time.Time)) >= touchInterval {
					if !seen {
						if entries.Add(1) > 10_000 {
							sweep(now)
						}
					}
					lastTouch.Store(userID, now)
					_ = userRepo.TouchLastActive(r.Context(), userID)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
