package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
)

// ToolCatalog — публичный (без авторизации) список участнических
// инструментов с ценой и описанием: нужен на экране регистрации, где
// человек ещё не авторизован.
func (h *CargoHandler) ToolCatalog(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ToolCatalog(r.Context())
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *CargoHandler) GetMyTools(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	items, err := h.svc.GetMyTools(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

type setMyToolsRequest struct {
	ToolIDs []uuid.UUID `json:"tool_ids"`
}

func (h *CargoHandler) SetMyTools(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	var req setMyToolsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	items, err := h.svc.SetMyTools(r.Context(), userID, req.ToolIDs)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}
