package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gandm/internal/auth"
	"gandm/internal/httpx"
)

func (h *AdminHandler) Analytics(w http.ResponseWriter, r *http.Request) {
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 0 {
		days = 7
	}

	analytics, err := h.svc.Analytics(r.Context(), days)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, analytics)
}

func (h *AdminHandler) ListModerators(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListModerators(r.Context())
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

type createModeratorRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AdminHandler) CreateModerator(w http.ResponseWriter, r *http.Request) {
	var req createModeratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	moderator, err := h.svc.CreateModerator(r.Context(), req.Email, req.Password)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, moderator)
}

// RequireAdminRole is chi middleware for staff routes that moderators must
// NOT reach (ТЗ §19.6: модератор — только верификация и просмотр
// участников). Runs after RequireAdminAuth.
func (h *AdminHandler) RequireAdminRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminID, ok := auth.AdminIDFromContext(r.Context())
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}
		if err := h.svc.RequireAdminRole(r.Context(), adminID); err != nil {
			writeAdminServiceError(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}
