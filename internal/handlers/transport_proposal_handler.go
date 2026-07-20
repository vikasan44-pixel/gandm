package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/models"
	"gandm/internal/service"
)

type proposalItemPayload struct {
	LengthM float64 `json:"length_m"`
	WidthM  float64 `json:"width_m"`
	HeightM float64 `json:"height_m"`
}

type sendTransportProposalRequest struct {
	CargoRequestID *uuid.UUID            `json:"cargo_request_id"`
	Origin         geoPointPayload       `json:"origin"`
	Destination    geoPointPayload       `json:"destination"`
	CargoName      string                `json:"cargo_name"`
	VolumeM3       float64               `json:"volume_m3"`
	WeightKg       float64               `json:"weight_kg"`
	PickupDate     string                `json:"pickup_date"`
	Currency       string                `json:"currency"`
	Items          []proposalItemPayload `json:"items"`
}

type priceRequest struct {
	Price float64 `json:"price"`
}

// SendTransportProposal — POST /api/transport/{vehicleId}/proposals.
func (h *CargoHandler) SendTransportProposal(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	vehicleID, err := uuid.Parse(chi.URLParam(r, "vehicleId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid vehicle id")
		return
	}

	var req sendTransportProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	items := make([]models.TransportProposalItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, models.TransportProposalItem{LengthM: it.LengthM, WidthM: it.WidthM, HeightM: it.HeightM})
	}

	proposal, err := h.svc.SendTransportProposal(r.Context(), userID, vehicleID, service.SendTransportProposalInput{
		CargoRequestID: req.CargoRequestID,
		Origin:         req.Origin.toModel(),
		Destination:    req.Destination.toModel(),
		CargoName:      req.CargoName,
		VolumeM3:       req.VolumeM3,
		WeightKg:       req.WeightKg,
		PickupDate:     req.PickupDate,
		Currency:       req.Currency,
		Items:          items,
	})
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, proposal)
}

// ListMyTransportProposals — GET /api/transport-proposals/mine (client view).
func (h *CargoHandler) ListMyTransportProposals(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	list, err := h.svc.ListMyTransportProposals(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

// ListIncomingTransportProposals — GET /api/transport-proposals/incoming (carrier view).
func (h *CargoHandler) ListIncomingTransportProposals(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	list, err := h.svc.ListIncomingTransportProposals(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, list)
}

func (h *CargoHandler) proposalID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid proposal id")
		return uuid.Nil, false
	}
	return id, true
}

func (h *CargoHandler) decodePrice(w http.ResponseWriter, r *http.Request) (float64, bool) {
	var req priceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return 0, false
	}
	return req.Price, true
}

// QuoteTransportProposal — POST /api/transport-proposals/{id}/quote (carrier).
func (h *CargoHandler) QuoteTransportProposal(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	id, ok := h.proposalID(w, r)
	if !ok {
		return
	}
	price, ok := h.decodePrice(w, r)
	if !ok {
		return
	}
	p, err := h.svc.QuoteTransportProposal(r.Context(), userID, id, price)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

// CounterTransportProposal — POST /api/transport-proposals/{id}/counter (client).
func (h *CargoHandler) CounterTransportProposal(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	id, ok := h.proposalID(w, r)
	if !ok {
		return
	}
	price, ok := h.decodePrice(w, r)
	if !ok {
		return
	}
	p, err := h.svc.CounterTransportProposal(r.Context(), userID, id, price)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

// FinalTransportProposal — POST /api/transport-proposals/{id}/final (carrier).
func (h *CargoHandler) FinalTransportProposal(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	id, ok := h.proposalID(w, r)
	if !ok {
		return
	}
	price, ok := h.decodePrice(w, r)
	if !ok {
		return
	}
	p, err := h.svc.FinalTransportProposal(r.Context(), userID, id, price)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

// AcceptTransportProposal — POST /api/transport-proposals/{id}/accept.
func (h *CargoHandler) AcceptTransportProposal(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	id, ok := h.proposalID(w, r)
	if !ok {
		return
	}
	view, err := h.svc.AcceptTransportProposal(r.Context(), userID, id)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, view)
}

// RejectTransportProposal — POST /api/transport-proposals/{id}/reject.
func (h *CargoHandler) RejectTransportProposal(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	id, ok := h.proposalID(w, r)
	if !ok {
		return
	}
	p, err := h.svc.RejectTransportProposal(r.Context(), userID, id)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}
