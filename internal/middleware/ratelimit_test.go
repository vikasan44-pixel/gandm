package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPerIPRateLimit(t *testing.T) {
	limiter := PerIPRateLimit(3, time.Minute)
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	do := func(remoteAddr string) int {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = remoteAddr
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 1; i <= 3; i++ {
		if code := do("10.0.0.1:1234"); code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i, code)
		}
	}
	if code := do("10.0.0.1:1234"); code != http.StatusTooManyRequests {
		t.Errorf("4th request: got %d, want 429", code)
	}

	// A different IP has its own budget.
	if code := do("10.0.0.2:1234"); code != http.StatusOK {
		t.Errorf("other IP: got %d, want 200", code)
	}
}

func TestPerIPRateLimitWindowReset(t *testing.T) {
	limiter := PerIPRateLimit(1, 10*time.Millisecond)
	handler := limiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	do := func() int {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "10.0.0.3:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := do(); code != http.StatusOK {
		t.Fatalf("first request: got %d, want 200", code)
	}
	if code := do(); code != http.StatusTooManyRequests {
		t.Fatalf("second request in window: got %d, want 429", code)
	}
	time.Sleep(15 * time.Millisecond)
	if code := do(); code != http.StatusOK {
		t.Errorf("after window reset: got %d, want 200", code)
	}
}
