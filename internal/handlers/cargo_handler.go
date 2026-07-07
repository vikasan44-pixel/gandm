package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/models"
	"gandm/internal/repository"
	"gandm/internal/service"
)

type CargoHandler struct {
	svc *service.CargoService
}

func NewCargoHandler(svc *service.CargoService) *CargoHandler {
	return &CargoHandler{svc: svc}
}

// geoPointPayload is the wire format for a picked map point. Coordinates
// must already be WGS-84 (the frontend converts Amap's GCJ-02 before
// submitting); source records provenance for debugging.
type geoPointPayload struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Label   string  `json:"label"`
	Source  string  `json:"source"`
	Country string  `json:"country"`
}

func (p geoPointPayload) toModel() models.GeoPoint {
	return models.GeoPoint{
		Lat:     p.Lat,
		Lng:     p.Lng,
		Label:   p.Label,
		Source:  models.CoordSource(p.Source),
		Country: p.Country,
	}
}

type createCargoRequestRequest struct {
	Origin      geoPointPayload `json:"origin"`
	Destination geoPointPayload `json:"destination"`
	VolumeM3    float64         `json:"volume_m3"`
	WeightKg    float64         `json:"weight_kg"`
	Description string          `json:"description"`
}

func (h *CargoHandler) CreateCargoRequest(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req createCargoRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	cargo, err := h.svc.CreateCargoRequest(r.Context(), userID, service.CreateCargoRequestInput{
		Origin:      req.Origin.toModel(),
		Destination: req.Destination.toModel(),
		VolumeM3:    req.VolumeM3,
		WeightKg:    req.WeightKg,
		Description: req.Description,
	})
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, cargo)
}

func (h *CargoHandler) ListMyCargoRequests(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	items, err := h.svc.ListMyCargoRequests(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *CargoHandler) ListAvailableCargoRequests(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	items, err := h.svc.ListAvailableCargoRequests(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

type createOfferRequest struct {
	Price                float64  `json:"price"`
	Conditions           string   `json:"conditions"`
	WarehouseFillPercent *float64 `json:"warehouse_fill_percent"`
}

func (h *CargoHandler) CreateOffer(w http.ResponseWriter, r *http.Request) {
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

	var req createOfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	offer, err := h.svc.CreateOffer(r.Context(), userID, cargoID, service.CreateOfferInput{
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

func (h *CargoHandler) ListOffersForCargo(w http.ResponseWriter, r *http.Request) {
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

	offers, err := h.svc.ListOffersForClient(r.Context(), userID, cargoID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, offers)
}

func writeCargoServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, service.ErrAccountNotEligible):
		httpx.WriteError(w, http.StatusForbidden, "account_not_eligible", "account status does not allow this action")
	case errors.Is(err, service.ErrForbiddenTool):
		httpx.WriteError(w, http.StatusForbidden, "tool_required", "this action requires a specific tool")
	case errors.Is(err, service.ErrForbiddenNotOwner):
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "you don't have access to this resource")
	case errors.Is(err, service.ErrCargoNotOpen):
		httpx.WriteError(w, http.StatusConflict, "cargo_not_open", "this cargo request is no longer open")
	case errors.Is(err, service.ErrContactLimitReached):
		httpx.WriteError(w, http.StatusTooManyRequests, "contact_limit_reached", "contact reveal limit reached for this account")
	case errors.Is(err, repository.ErrAlreadyRevealed):
		httpx.WriteError(w, http.StatusConflict, "already_revealed", "contact already revealed for this cargo request")
	case errors.Is(err, service.ErrAlreadyResponded):
		httpx.WriteError(w, http.StatusConflict, "already_responded", "this consolidation suggestion is already resolved")
	case errors.Is(err, service.ErrRouteExists):
		httpx.WriteError(w, http.StatusConflict, "route_exists", "this route is already added")
	case errors.Is(err, repository.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "not found")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
