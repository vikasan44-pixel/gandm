package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"gandm/internal/httpx"
)

// PerIPRateLimit is a fixed-window in-memory limiter for unauthenticated
// endpoints (login/refresh): at most n requests per window per client IP.
// State is per-process, which matches the current single-binary deployment;
// a multi-instance setup would need a shared store instead.
func PerIPRateLimit(n int, window time.Duration) func(http.Handler) http.Handler {
	type bucket struct {
		windowStart time.Time
		count       int
	}
	var mu sync.Mutex
	buckets := make(map[string]*bucket)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// chi's RealIP middleware rewrites RemoteAddr to the bare
			// client IP; without it RemoteAddr is host:port.
			ip := r.RemoteAddr
			if host, _, err := net.SplitHostPort(ip); err == nil {
				ip = host
			}

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
