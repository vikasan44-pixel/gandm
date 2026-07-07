package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/repository"
)

// RequireTool gates access purely on tool possession — never on
// participant_type — per the platform's core authorization principle. The
// required tool key is read from the named chi URL parameter, so one
// middleware instance can protect many routes keyed by different tools.
// Must run after auth.RequireAuth.
func RequireTool(toolRepo *repository.ToolRepository, keyParam string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := auth.UserIDFromContext(r.Context())
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
				return
			}

			key := chi.URLParam(r, keyParam)
			has, err := toolRepo.UserHasTool(r.Context(), userID, key)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
				return
			}
			if !has {
				httpx.WriteError(w, http.StatusForbidden, "tool_required", "this action requires the \""+key+"\" tool")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
