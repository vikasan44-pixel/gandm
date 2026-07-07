package middleware

import (
	"net/http"

	"gandm/internal/auth"
	"gandm/internal/repository"
)

// TouchLastActive stamps users.last_active_at on every authenticated
// request. Must run after auth.RequireAuth so the user id is in context.
// Errors are ignored — a missed activity timestamp isn't worth failing the
// request over.
func TouchLastActive(userRepo *repository.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID, ok := auth.UserIDFromContext(r.Context()); ok {
				_ = userRepo.TouchLastActive(r.Context(), userID)
			}
			next.ServeHTTP(w, r)
		})
	}
}
