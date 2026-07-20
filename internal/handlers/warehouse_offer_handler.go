package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
)

// ListCargoForMyWarehouses — GET /api/warehouse/available-cargo (warehouse owner).
func (h *CargoHandler) ListCargoForMyWarehouses(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	list, err := h.svc.ListCargoForMyWarehouses(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

type submitWarehouseOfferRequest struct {
	WarehouseID uuid.UUID `json:"warehouse_id"`
	Price       float64   `json:"price"`
	Currency    string    `json:"currency"`
	Conditions  string    `json:"conditions"`
}

// SubmitWarehouseOffer — POST /api/cargo/{id}/warehouse-offers (warehouse owner bids).
func (h *CargoHandler) SubmitWarehouseOffer(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	cargoID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid cargo id")
		return
	}
	var req submitWarehouseOfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	offer, err := h.svc.SubmitWarehouseOffer(r.Context(), userID, cargoID, req.WarehouseID, req.Price, req.Currency, req.Conditions)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, offer)
}

// ListWarehouseOffersForCargo — GET /api/cargo/{id}/warehouse-offers (cargo owner).
func (h *CargoHandler) ListWarehouseOffersForCargo(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	cargoID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid cargo id")
		return
	}
	list, err := h.svc.ListWarehouseOffersForCargo(r.Context(), userID, cargoID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// SelectWarehouseOffer — POST /api/cargo/{id}/warehouse-offers/{offerId}/select.
func (h *CargoHandler) SelectWarehouseOffer(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	cargoID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid cargo id")
		return
	}
	offerID, err := uuid.Parse(chi.URLParam(r, "offerId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid offer id")
		return
	}
	res, err := h.svc.SelectWarehouseOffer(r.Context(), userID, cargoID, offerID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// --- Phase 3: warehouse offers on consolidated requests ---

// ListConsolidatedForMyWarehouses — GET /api/warehouse/available-consolidated.
func (h *CargoHandler) ListConsolidatedForMyWarehouses(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	list, err := h.svc.ListConsolidatedForMyWarehouses(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// SubmitWarehouseOfferForConsolidated — POST /api/consolidated/{id}/warehouse-offers.
func (h *CargoHandler) SubmitWarehouseOfferForConsolidated(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	consolidatedID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid consolidated id")
		return
	}
	var req submitWarehouseOfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	offer, err := h.svc.SubmitWarehouseOfferForConsolidated(r.Context(), userID, consolidatedID, req.WarehouseID, req.Price, req.Currency, req.Conditions)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, offer)
}

// ListWarehouseOffersForConsolidated — GET /api/consolidated/{id}/warehouse-offers.
func (h *CargoHandler) ListWarehouseOffersForConsolidated(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	consolidatedID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid consolidated id")
		return
	}
	list, err := h.svc.ListWarehouseOffersForConsolidated(r.Context(), userID, consolidatedID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// SelectWarehouseOfferForConsolidated — POST /api/consolidated/{id}/warehouse-offers/{offerId}/select.
func (h *CargoHandler) SelectWarehouseOfferForConsolidated(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	consolidatedID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid consolidated id")
		return
	}
	offerID, err := uuid.Parse(chi.URLParam(r, "offerId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid offer id")
		return
	}
	res, err := h.svc.SelectWarehouseOfferForConsolidated(r.Context(), userID, consolidatedID, offerID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

type joinConsolidationRequest struct {
	CargoRequestID uuid.UUID `json:"cargo_request_id"`
}

// JoinConsolidation — POST /api/consolidated/{id}/join. A client whose matching
// cargo missed the window joins an existing open consolidation.
func (h *CargoHandler) JoinConsolidation(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	consolidatedID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid consolidated id")
		return
	}
	var req joinConsolidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	cons, err := h.svc.RequestJoinConsolidation(r.Context(), userID, consolidatedID, req.CargoRequestID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cons)
}

// ListMatchingConsolidationsForCargo — GET /api/cargo/{id}/matching-consolidations.
func (h *CargoHandler) ListMatchingConsolidationsForCargo(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	cargoID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid cargo id")
		return
	}
	list, err := h.svc.ListMatchingConsolidationsForCargo(r.Context(), userID, cargoID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}
