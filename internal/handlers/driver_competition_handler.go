package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
)

type createDriverCompetitionRequest struct {
	RouteID      uuid.UUID `json:"route_id"`
	VolumeM3     float64   `json:"volume_m3"`
	DispatchDate string    `json:"dispatch_date"`
}

func (h *CargoHandler) CreateDriverCompetition(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req createDriverCompetitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RouteID == uuid.Nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "route_id is required")
		return
	}

	competition, err := h.svc.CreateDriverCompetition(r.Context(), userID, req.RouteID, req.VolumeM3, req.DispatchDate)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, competition)
}

func (h *CargoHandler) UpdateDriverCompetition(w http.ResponseWriter, r *http.Request) {
	competitionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid competition id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	var req createDriverCompetitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RouteID == uuid.Nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "route_id is required")
		return
	}
	competition, err := h.svc.UpdateDriverCompetition(r.Context(), userID, competitionID, req.RouteID, req.VolumeM3, req.DispatchDate)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, competition)
}

func (h *CargoHandler) CancelDriverCompetition(w http.ResponseWriter, r *http.Request) {
	competitionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid competition id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.CancelDriverCompetition(r.Context(), userID, competitionID); err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "closed"})
}

func (h *CargoHandler) ListMyDriverCompetitions(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	items, err := h.svc.ListMyDriverCompetitions(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *CargoHandler) ListOpenDriverCompetitions(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	items, err := h.svc.ListOpenDriverCompetitions(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *CargoHandler) ListMyDriverCompetitionResponses(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	items, err := h.svc.ListMyDriverCompetitionResponses(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

type driverBidRequest struct {
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	Comment  string  `json:"comment"`
}

func (h *CargoHandler) CreateDriverBid(w http.ResponseWriter, r *http.Request) {
	competitionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid competition id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req driverBidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	bid, err := h.svc.CreateDriverBid(r.Context(), userID, competitionID, req.Price, req.Currency, req.Comment)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, bid)
}

func (h *CargoHandler) UpdateMyDriverBid(w http.ResponseWriter, r *http.Request) {
	bidID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid bid id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	var req driverBidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	bid, err := h.svc.UpdateMyDriverBid(r.Context(), userID, bidID, req.Price, req.Currency, req.Comment)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, bid)
}

func (h *CargoHandler) WithdrawMyDriverBid(w http.ResponseWriter, r *http.Request) {
	bidID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid bid id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	bid, err := h.svc.WithdrawMyDriverBid(r.Context(), userID, bidID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, bid)
}

func (h *CargoHandler) SelectDriverBid(w http.ResponseWriter, r *http.Request) {
	competitionID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid competition id")
		return
	}
	bidID, err := uuid.Parse(chi.URLParam(r, "bid"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid bid id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	result, err := h.svc.SelectDriverBid(r.Context(), userID, competitionID, bidID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}
