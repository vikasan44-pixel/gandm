package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"gandm/internal/httpx"
)

type contextKey string

const (
	userIDContextKey  contextKey = "user_id"
	adminIDContextKey contextKey = "admin_id"
)

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	return token, ok && token != ""
}

// RequireAuth identifies the caller from a Bearer access token issued to a
// participant account. It only proves "this request belongs to this user" —
// it grants no permissions by itself. Authorization for protected business
// actions must still go through tools, not this middleware.
func (m *Manager) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}

		userID, err := m.ParseAccessToken(token, SubjectUser)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdminAuth identifies the caller from a Bearer access token issued to
// an admin/moderator account. These are staff accounts, structurally separate
// from participants — a user token is rejected here and vice versa.
func (m *Manager) RequireAdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}

		adminID, err := m.ParseAccessToken(token, SubjectAdmin)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), adminIDContextKey, adminID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDContextKey).(uuid.UUID)
	return id, ok
}

func AdminIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(adminIDContextKey).(uuid.UUID)
	return id, ok
}
