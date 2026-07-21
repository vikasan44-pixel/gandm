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
	Lat     float64           `json:"lat"`
	Lng     float64           `json:"lng"`
	Label   string            `json:"label"`
	Source  string            `json:"source"`
	Country string            `json:"country"`
	Labels  map[string]string `json:"labels"`
}

func (p geoPointPayload) toModel() models.GeoPoint {
	return models.GeoPoint{
		Lat:     p.Lat,
		Lng:     p.Lng,
		Label:   p.Label,
		Source:  models.CoordSource(p.Source),
		Country: p.Country,
		Labels:  p.Labels,
	}
}

type createCargoRequestRequest struct {
	Origin      geoPointPayload `json:"origin"`
	Destination geoPointPayload `json:"destination"`
	VolumeM3    float64         `json:"volume_m3"`
	WeightKg    float64         `json:"weight_kg"`
	Category    string          `json:"category"`
	Description string          `json:"description"`

	Packaging   string                `json:"packaging"`
	PlacesCount int                   `json:"places_count"`
	Stackable   bool                  `json:"stackable"`
	ADRRequired bool                  `json:"adr_required"`
	Items       []proposalItemPayload `json:"items"`
}

func (req createCargoRequestRequest) toInput() service.CreateCargoRequestInput {
	items := make([]models.CargoRequestItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, models.CargoRequestItem{LengthM: it.LengthM, WidthM: it.WidthM, HeightM: it.HeightM})
	}
	return service.CreateCargoRequestInput{
		Origin:      req.Origin.toModel(),
		Destination: req.Destination.toModel(),
		VolumeM3:    req.VolumeM3,
		WeightKg:    req.WeightKg,
		Category:    models.CargoCategory(req.Category),
		Description: req.Description,
		Packaging:   models.CargoPackaging(req.Packaging),
		PlacesCount: req.PlacesCount,
		Stackable:   req.Stackable,
		ADRRequired: req.ADRRequired,
		Items:       items,
	}
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

	cargo, err := h.svc.CreateCargoRequest(r.Context(), userID, req.toInput())
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, cargo)
}

func (h *CargoHandler) UpdateCargoRequest(w http.ResponseWriter, r *http.Request) {
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
	var req createCargoRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	cargo, err := h.svc.UpdateCargoRequest(r.Context(), userID, cargoID, req.toInput())
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cargo)
}

func (h *CargoHandler) CancelCargoRequest(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.CancelCargoRequest(r.Context(), userID, cargoID); err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "closed"})
}

func (h *CargoHandler) ListMyCargoRequests(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	pageRequest, err := pageRequestFromQuery(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	items, err := h.svc.ListMyCargoRequests(r.Context(), userID, pageRequest)
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

	from, err := geoPointFromQuery(r, "from")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	to, err := geoPointFromQuery(r, "to")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	pageRequest, err := pageRequestFromQuery(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	items, err := h.svc.ListAvailableCargoRequests(
		r.Context(),
		userID,
		from,
		to,
		pageRequest,
	)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *CargoHandler) ListMyCargoCompetitionResponses(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	items, err := h.svc.ListMyCargoCompetitionResponses(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

type createOfferRequest struct {
	Price                float64  `json:"price"`
	Currency             string   `json:"currency"`
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
		Currency:             req.Currency,
		Conditions:           req.Conditions,
		WarehouseFillPercent: req.WarehouseFillPercent,
	})
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, offer)
}

func (h *CargoHandler) UpdateMyOffer(w http.ResponseWriter, r *http.Request) {
	offerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid offer id")
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
	offer, err := h.svc.UpdateMyOffer(r.Context(), userID, offerID, service.CreateOfferInput{
		Price: req.Price, Currency: req.Currency, Conditions: req.Conditions, WarehouseFillPercent: req.WarehouseFillPercent,
	})
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, offer)
}

func (h *CargoHandler) WithdrawMyOffer(w http.ResponseWriter, r *http.Request) {
	offerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid offer id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	offer, err := h.svc.WithdrawMyOffer(r.Context(), userID, offerID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, offer)
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
	case errors.Is(err, service.ErrOfferNotEditable):
		httpx.WriteError(w, http.StatusConflict, "offer_not_editable", "this offer can no longer be changed")
	case errors.Is(err, service.ErrContactLimitReached):
		httpx.WriteError(w, http.StatusTooManyRequests, "contact_limit_reached", "contact reveal limit reached for this account")
	case errors.Is(err, repository.ErrAlreadyRevealed):
		httpx.WriteError(w, http.StatusConflict, "already_revealed", "contact already revealed for this cargo request")
	case errors.Is(err, service.ErrAlreadyResponded):
		httpx.WriteError(w, http.StatusConflict, "already_responded", "this consolidation suggestion is already resolved")
	case errors.Is(err, service.ErrSubscriptionRequired):
		httpx.WriteError(w, http.StatusForbidden, "subscription_required", "a subscription is required to initiate consolidation")
	case errors.Is(err, service.ErrPaymentRequired):
		httpx.WriteError(w, http.StatusPaymentRequired, "payment_required", "a subscription or one-time payment is required to accept")
	case errors.Is(err, service.ErrInviteWrongState):
		httpx.WriteError(w, http.StatusConflict, "invite_wrong_state", "the consolidation invite is not in the required state")
	case errors.Is(err, service.ErrNotInvitedClient):
		httpx.WriteError(w, http.StatusForbidden, "not_invited", "only the invited client can perform this action")
	case errors.Is(err, service.ErrConsolidationNotAccepted):
		httpx.WriteError(w, http.StatusConflict, "not_accepted", "the consolidation must be accepted before selecting a carrier")
	case errors.Is(err, repository.ErrAlreadyPaid):
		httpx.WriteError(w, http.StatusConflict, "already_paid", "this consolidation is already paid")
	case errors.Is(err, service.ErrAlreadyRated):
		httpx.WriteError(w, http.StatusConflict, "already_rated", "you already rated this counterparty for this deal")
	case errors.Is(err, service.ErrNoDealBetween):
		httpx.WriteError(w, http.StatusForbidden, "no_deal", "rating requires a completed deal between the two users")
	case errors.Is(err, service.ErrUnsupportedFile):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "unsupported_file", "unsupported file type")
	case errors.Is(err, service.ErrFileTooLarge):
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large", "file exceeds maximum size")
	case errors.Is(err, service.ErrRouteExists):
		httpx.WriteError(w, http.StatusConflict, "route_exists", "this route is already added")
	case errors.Is(err, service.ErrVehicleLimitReached):
		httpx.WriteError(w, http.StatusConflict, "vehicle_limit_reached", "vehicle limit reached for this account")
	case errors.Is(err, service.ErrVehicleTripOriginTooFar):
		httpx.WriteError(w, http.StatusBadRequest, "vehicle_trip_origin_too_far", "trip origin must be within the matching radius of the vehicle location")
	case errors.Is(err, repository.ErrVehicleIdentityTaken):
		httpx.WriteError(w, http.StatusConflict, "vehicle_identity_taken", "vehicle plate or VIN is already registered")
	case errors.Is(err, repository.ErrVehicleTripDateConflict):
		httpx.WriteError(w, http.StatusConflict, "vehicle_trip_date_conflict", "this vehicle already has a trip on the selected date")
	case errors.Is(err, repository.ErrVehicleActiveTrip):
		httpx.WriteError(w, http.StatusConflict, "vehicle_active_trip_conflict", "this vehicle already has an active trip")
	case errors.Is(err, service.ErrConsolidationNotMatched):
		httpx.WriteError(w, http.StatusConflict, "not_matched", "the consolidation is not matched yet")
	case errors.Is(err, service.ErrCustomsAlreadySelected):
		httpx.WriteError(w, http.StatusConflict, "customs_already_selected", "a customs representative is already selected")
	case errors.Is(err, repository.ErrAlreadyOffered):
		httpx.WriteError(w, http.StatusConflict, "already_offered", "you already submitted an offer for this consolidation")
	case errors.Is(err, repository.ErrAlreadyBid):
		httpx.WriteError(w, http.StatusConflict, "already_bid", "you already bid on this competition")
	case errors.Is(err, service.ErrCompetitionClosed):
		httpx.WriteError(w, http.StatusConflict, "competition_closed", "this competition is already closed")
	case errors.Is(err, repository.ErrOpenCompetitionExists):
		httpx.WriteError(w, http.StatusConflict, "competition_exists", "an open competition already exists for this route")
	case errors.Is(err, service.ErrNoVehicles):
		httpx.WriteError(w, http.StatusConflict, "no_vehicles", "add at least one vehicle to bid")
	case errors.Is(err, service.ErrEmployeeOfEmployee):
		httpx.WriteError(w, http.StatusForbidden, "employee_of_employee", "employees cannot create their own employees")
	case errors.Is(err, service.ErrProposalNotParty):
		httpx.WriteError(w, http.StatusForbidden, "proposal_not_party", "you are not a party to this proposal")
	case errors.Is(err, service.ErrProposalWrongState):
		httpx.WriteError(w, http.StatusConflict, "proposal_wrong_state", "proposal is not in a state that allows this action")
	case errors.Is(err, service.ErrProposalToOwnVehicle):
		httpx.WriteError(w, http.StatusBadRequest, "proposal_own_vehicle", "cannot send a proposal to your own vehicle")
	case errors.Is(err, service.ErrConsolidationRouteMismatch):
		httpx.WriteError(w, http.StatusConflict, "consolidation_route_mismatch", "cargo route does not match this consolidation")
	case errors.Is(err, service.ErrWarehouseNotEligibleForCargo):
		httpx.WriteError(w, http.StatusConflict, "warehouse_not_eligible", "this warehouse cannot collect this cargo")
	case errors.Is(err, service.ErrWarehouseNotPublished):
		httpx.WriteError(w, http.StatusConflict, "warehouse_not_published", "warehouse must be published with pickup enabled")
	case errors.Is(err, service.ErrWarehouseOfferWrongState):
		httpx.WriteError(w, http.StatusConflict, "warehouse_offer_wrong_state", "warehouse offer is not in the required state")
	case errors.Is(err, repository.ErrWarehouseAlreadyOffered):
		httpx.WriteError(w, http.StatusConflict, "warehouse_already_offered", "this warehouse already offered on this cargo")
	case errors.Is(err, repository.ErrEmailTaken):
		httpx.WriteError(w, http.StatusConflict, "email_taken", "email is already registered")
	case errors.Is(err, repository.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "not found")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
