package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/service"
)

func (h *AdminHandler) ListPermissionSets(w http.ResponseWriter, r *http.Request) {
	sets, err := h.svc.ListPermissionSets(r.Context())
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sets)
}

type createPermissionSetRequest struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	PriceKZT    float64     `json:"price_kzt"`
	ToolIDs     []uuid.UUID `json:"tool_ids"`
}

func (h *AdminHandler) CreatePermissionSet(w http.ResponseWriter, r *http.Request) {
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req createPermissionSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	set, err := h.svc.CreatePermissionSet(r.Context(), adminID, service.CreatePermissionSetInput{
		Name:        req.Name,
		Description: req.Description,
		PriceKZT:    req.PriceKZT,
		ToolIDs:     req.ToolIDs,
	})
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, set)
}

type updatePermissionSetRequest struct {
	Name        *string      `json:"name"`
	Description *string      `json:"description"`
	PriceKZT    *float64     `json:"price_kzt"`
	ToolIDs     *[]uuid.UUID `json:"tool_ids"`
}

func (h *AdminHandler) UpdatePermissionSet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid permission set id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req updatePermissionSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	set, err := h.svc.UpdatePermissionSet(r.Context(), adminID, id, service.PermissionSetPatch{
		Name:        req.Name,
		Description: req.Description,
		PriceKZT:    req.PriceKZT,
		ToolIDs:     req.ToolIDs,
	})
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, set)
}
