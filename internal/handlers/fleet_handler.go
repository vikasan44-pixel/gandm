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

type vehicleRequest struct {
	Axles      int     `json:"axles"`
	CapacityKg float64 `json:"capacity_kg"`
	CapacityM3 float64 `json:"capacity_m3"`
	LengthM    float64 `json:"length_m"`
	WidthM     float64 `json:"width_m"`
	HeightM    float64 `json:"height_m"`
	BodyType   string  `json:"body_type"`
	// Опциональное местонахождение координатами (по карте) — «откуда».
	Location *geoPointPayload `json:"location"`
	// Ноль или несколько назначений (координатами) — «куда».
	Destinations []geoPointPayload `json:"destinations"`
}

func (p *geoPointPayload) toModelPtr() *models.GeoPoint {
	if p == nil {
		return nil
	}
	m := p.toModel()
	return &m
}

func geoPayloadsToModels(ps []geoPointPayload) []models.GeoPoint {
	out := make([]models.GeoPoint, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.toModel())
	}
	return out
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
		Axles:        req.Axles,
		CapacityKg:   req.CapacityKg,
		CapacityM3:   req.CapacityM3,
		LengthM:      req.LengthM,
		WidthM:       req.WidthM,
		HeightM:      req.HeightM,
		BodyType:     req.BodyType,
		Location:     req.Location.toModelPtr(),
		Destinations: geoPayloadsToModels(req.Destinations),
	})
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, vehicle)
}

// vehicleLocationRequest carries an optional map point; null clears it.
type vehicleLocationRequest struct {
	Location *geoPointPayload `json:"location"`
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

	vehicle, err := h.svc.UpdateMyVehicleLocation(r.Context(), userID, vehicleID, req.Location.toModelPtr())
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, vehicle)
}

type vehicleDestinationRequest struct {
	Point geoPointPayload `json:"point"`
}

func (h *CargoHandler) AddMyVehicleDestination(w http.ResponseWriter, r *http.Request) {
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
	var req vehicleDestinationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	dest, err := h.svc.AddMyVehicleDestination(r.Context(), userID, vehicleID, req.Point.toModel())
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, dest)
}

func (h *CargoHandler) DeleteMyVehicleDestination(w http.ResponseWriter, r *http.Request) {
	vehicleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid vehicle id")
		return
	}
	destID, err := uuid.Parse(chi.URLParam(r, "did"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid destination id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.DeleteMyVehicleDestination(r.Context(), userID, vehicleID, destID); err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
