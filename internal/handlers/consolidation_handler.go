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

func parseConsolidationIDs(r *http.Request) (cargoID, suggestionID uuid.UUID, ok bool) {
	cargoID, errA := uuid.Parse(chi.URLParam(r, "id"))
	suggestionID, errB := uuid.Parse(chi.URLParam(r, "sid"))
	return cargoID, suggestionID, errA == nil && errB == nil
}

func (h *CargoHandler) GetConsolidation(w http.ResponseWriter, r *http.Request) {
	cargoID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid cargo request id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	view, err := h.svc.GetActiveConsolidation(r.Context(), userID, cargoID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	// view is nil when there is no pending suggestion — an explicit JSON
	// null keeps the "нет предложения" case unambiguous for the frontend.
	httpx.WriteJSON(w, http.StatusOK, view)
}

func (h *CargoHandler) AgreeConsolidation(w http.ResponseWriter, r *http.Request) {
	cargoID, suggestionID, ok := parseConsolidationIDs(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid cargo or suggestion id")
		return
	}
	userID, okAuth := auth.UserIDFromContext(r.Context())
	if !okAuth {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	suggestion, err := h.svc.AgreeConsolidation(r.Context(), userID, cargoID, suggestionID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, suggestion)
}

func (h *CargoHandler) DeclineConsolidation(w http.ResponseWriter, r *http.Request) {
	cargoID, suggestionID, ok := parseConsolidationIDs(r)
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid cargo or suggestion id")
		return
	}
	userID, okAuth := auth.UserIDFromContext(r.Context())
	if !okAuth {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if err := h.svc.DeclineConsolidation(r.Context(), userID, cargoID, suggestionID); err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "declined"})
}

func (h *CargoHandler) ListMyConsolidated(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	items, err := h.svc.ListMyConsolidated(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *CargoHandler) ListAvailableConsolidated(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	items, err := h.svc.ListAvailableConsolidated(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *CargoHandler) CreateConsolidatedOffer(w http.ResponseWriter, r *http.Request) {
	consolidatedID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid consolidated request id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req createOfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	offer, err := h.svc.CreateConsolidatedOffer(r.Context(), userID, consolidatedID, service.CreateOfferInput{
		Price:                req.Price,
		Conditions:           req.Conditions,
		WarehouseFillPercent: req.WarehouseFillPercent,
	})
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, offer)
}

func (h *CargoHandler) ListConsolidatedOffers(w http.ResponseWriter, r *http.Request) {
	consolidatedID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid consolidated request id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	offers, err := h.svc.ListConsolidatedOffersForClient(r.Context(), userID, consolidatedID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, offers)
}
