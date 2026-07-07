package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
)

func (h *AdminHandler) ListUserRoutes(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}

	routes, err := h.svc.ListUserRoutes(r.Context(), userID)
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, routes)
}

func (h *AdminHandler) AddUserRoute(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req routeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	route, err := h.svc.AddUserRoute(r.Context(), adminID, userID, req.Origin.toModel(), req.Destination.toModel())
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, route)
}

func (h *AdminHandler) DeleteUserRoute(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	routeID, err := uuid.Parse(chi.URLParam(r, "routeId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid route id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if err := h.svc.DeleteUserRoute(r.Context(), adminID, userID, routeID); err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
