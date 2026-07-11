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

func (h *AdminHandler) ListTools(w http.ResponseWriter, r *http.Request) {
	tools, err := h.svc.ListTools(r.Context())
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tools)
}

type createToolRequest struct {
	Key         string  `json:"key"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	PriceKZT    float64 `json:"price_kzt"`
}

func (h *AdminHandler) CreateTool(w http.ResponseWriter, r *http.Request) {
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req createToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	tool, err := h.svc.CreateTool(r.Context(), adminID, service.CreateToolInput{
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		PriceKZT:    req.PriceKZT,
	})
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, tool)
}

type updateToolRequest struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	Category    *string  `json:"category"`
	IsActive    *bool    `json:"is_active"`
	PriceKZT    *float64 `json:"price_kzt"`
}

func (h *AdminHandler) UpdateTool(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid tool id")
		return
	}
	adminID, ok := auth.AdminIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req updateToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	tool, err := h.svc.UpdateTool(r.Context(), adminID, id, service.ToolPatch{
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		IsActive:    req.IsActive,
		PriceKZT:    req.PriceKZT,
	})
	if err != nil {
		writeAdminServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tool)
}
