package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/models"
)

// SearchWarehouses — GET /api/warehouses/search?lat=&lng=&radius_km=. Returns
// published warehouses near the point (without contacts).
func (h *CargoHandler) SearchWarehouses(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	lat, err1 := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lng, err2 := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
	radius, err3 := strconv.ParseFloat(r.URL.Query().Get("radius_km"), 64)
	if err1 != nil || err2 != nil || err3 != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_query", "lat, lng and radius_km must be numbers")
		return
	}
	pageRequest, err := pageRequestFromQuery(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	cards, err := h.svc.SearchWarehouses(r.Context(), userID, lat, lng, radius, pageRequest)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cards)
}

func (h *CargoHandler) ListMyWarehouses(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	items, err := h.svc.ListMyWarehouses(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *CargoHandler) CreateMyWarehouse(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	var input models.Warehouse
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	item, err := h.svc.CreateMyWarehouse(r.Context(), userID, input)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, item)
}

func (h *CargoHandler) UpdateMyWarehouse(w http.ResponseWriter, r *http.Request) {
	warehouseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid warehouse id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	var input models.Warehouse
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	item, err := h.svc.UpdateMyWarehouse(r.Context(), userID, warehouseID, input)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, item)
}

func (h *CargoHandler) DeleteMyWarehouse(w http.ResponseWriter, r *http.Request) {
	warehouseID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid warehouse id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.DeleteMyWarehouse(r.Context(), userID, warehouseID); err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
