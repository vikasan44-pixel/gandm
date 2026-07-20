package middleware

import (
	"context"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"gandm/internal/httpx"
)

func clientIP(r *http.Request) string {
	// chi's RealIP middleware rewrites RemoteAddr to the bare client IP;
	// without it RemoteAddr is host:port.
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	return ip
}

// PerIPRateLimitDB — лимитер логина на Postgres: счётчики общие для всех
// инстансов бэкенда, горизонтальное масштабирование не размывает лимит.
// Отказ БД читается как «не лимитировано» (fail-open): недоступный логин
// хуже, чем окно для перебора при уже лежащей БД. Старые записи удаляются
// вероятностно (~1% запросов), без фонового воркера.
func PerIPRateLimitDB(db *pgxpool.Pool, n int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			ctx := r.Context()

			// Serialize attempts for the same IP. A separate SELECT followed by
			// INSERT lets concurrent requests all observe the same old count and
			// pass together. The transaction-scoped advisory lock makes the
			// decision and the insert one atomic operation across API instances.
			tx, err := db.Begin(ctx)
			if err != nil {
				log.Printf("rate limit: begin for %s: %v", ip, err)
				next.ServeHTTP(w, r)
				return
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, ip); err != nil {
				log.Printf("rate limit: lock for %s: %v", ip, err)
				next.ServeHTTP(w, r)
				return
			}

			var count int
			err = tx.QueryRow(ctx,
				`SELECT count(*) FROM login_attempts WHERE ip = $1 AND attempted_at > now() - $2::interval`,
				ip, window.String()).Scan(&count)
			if err != nil {
				log.Printf("rate limit: count for %s: %v", ip, err)
				next.ServeHTTP(w, r)
				return
			}
			if count >= n {
				httpx.WriteError(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
				return
			}

			if _, err := tx.Exec(ctx, `INSERT INTO login_attempts (ip) VALUES ($1)`, ip); err != nil {
				log.Printf("rate limit: record attempt for %s: %v", ip, err)
				next.ServeHTTP(w, r)
				return
			}
			if err := tx.Commit(ctx); err != nil {
				log.Printf("rate limit: commit for %s: %v", ip, err)
				next.ServeHTTP(w, r)
				return
			}
			if rand.Intn(100) == 0 {
				go func() {
					cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_, _ = db.Exec(cleanupCtx,
						`DELETE FROM login_attempts WHERE attempted_at < now() - $1::interval`, (2 * window).String())
				}()
			}

			next.ServeHTTP(w, r)
		})
	}
}

// PerIPRateLimit is a fixed-window in-memory limiter for unauthenticated
// endpoints: at most n requests per window per client IP. Per-process —
// kept for tests and single-binary setups; production wiring uses
// PerIPRateLimitDB above.
func PerIPRateLimit(n int, window time.Duration) func(http.Handler) http.Handler {
	type bucket struct {
		windowStart time.Time
		count       int
	}
	var mu sync.Mutex
	buckets := make(map[string]*bucket)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			now := time.Now()
			mu.Lock()
			b := buckets[ip]
			if b == nil || now.Sub(b.windowStart) >= window {
				// Opportunistic cleanup keeps the map bounded without a
				// background goroutine.
				if len(buckets) > 10_000 {
					for k, v := range buckets {
						if now.Sub(v.windowStart) >= window {
							delete(buckets, k)
						}
					}
				}
				b = &bucket{windowStart: now}
				buckets[ip] = b
			}
			b.count++
			limited := b.count > n
			mu.Unlock()

			if limited {
				httpx.WriteError(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
