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

type vehicleRequest struct {
	Axles           int     `json:"axles"`
	CapacityKg      float64 `json:"capacity_kg"`
	LengthM         float64 `json:"length_m"`
	WidthM          float64 `json:"width_m"`
	HeightM         float64 `json:"height_m"`
	BodyType        string  `json:"body_type"`
	CurrentLocation string  `json:"current_location"`
}

func (h *CargoHandler) ListMyVehicles(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	items, err := h.svc.ListMyVehicles(r.Context(), userID)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items)
}

func (h *CargoHandler) AddMyVehicle(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req vehicleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	vehicle, err := h.svc.AddMyVehicle(r.Context(), userID, service.VehicleInput{
		Axles:           req.Axles,
		CapacityKg:      req.CapacityKg,
		LengthM:         req.LengthM,
		WidthM:          req.WidthM,
		HeightM:         req.HeightM,
		BodyType:        req.BodyType,
		CurrentLocation: req.CurrentLocation,
	})
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, vehicle)
}

type vehicleLocationRequest struct {
	CurrentLocation string `json:"current_location"`
}

func (h *CargoHandler) UpdateMyVehicleLocation(w http.ResponseWriter, r *http.Request) {
	vehicleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid vehicle id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req vehicleLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	vehicle, err := h.svc.UpdateMyVehicleLocation(r.Context(), userID, vehicleID, req.CurrentLocation)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, vehicle)
}

func (h *CargoHandler) DeleteMyVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid vehicle id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	if err := h.svc.DeleteMyVehicle(r.Context(), userID, vehicleID); err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
