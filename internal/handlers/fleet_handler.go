package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/models"
	"gandm/internal/service"
)

type vehicleRequest struct {
	Name                string  `json:"name"`
	Axles               int     `json:"axles"`
	CapacityKg          float64 `json:"capacity_kg"`
	CapacityM3          float64 `json:"capacity_m3"`
	LengthM             float64 `json:"length_m"`
	WidthM              float64 `json:"width_m"`
	HeightM             float64 `json:"height_m"`
	BodyType            string  `json:"body_type"`
	RegistrationCountry string  `json:"registration_country"`
	PlateNumber         string  `json:"plate_number"`
	VIN                 string  `json:"vin"`
	PrivacyConsent      bool    `json:"privacy_consent"`
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
		Name:                req.Name,
		Axles:               req.Axles,
		CapacityKg:          req.CapacityKg,
		CapacityM3:          req.CapacityM3,
		LengthM:             req.LengthM,
		WidthM:              req.WidthM,
		HeightM:             req.HeightM,
		BodyType:            req.BodyType,
		RegistrationCountry: req.RegistrationCountry,
		PlateNumber:         req.PlateNumber,
		VIN:                 req.VIN,
		PrivacyConsent:      req.PrivacyConsent,
		Location:            req.Location.toModelPtr(),
		Destinations:        geoPayloadsToModels(req.Destinations),
	})
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, vehicle)
}

type vehicleNameRequest struct {
	Name string `json:"name"`
}

type vehicleDetailsRequest struct {
	Name       string  `json:"name"`
	Axles      int     `json:"axles"`
	CapacityKg float64 `json:"capacity_kg"`
	CapacityM3 float64 `json:"capacity_m3"`
	LengthM    float64 `json:"length_m"`
	WidthM     float64 `json:"width_m"`
	HeightM    float64 `json:"height_m"`
	BodyType   string  `json:"body_type"`
}

func (h *CargoHandler) UpdateMyVehicleDetails(w http.ResponseWriter, r *http.Request) {
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
	var req vehicleDetailsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	vehicle, err := h.svc.UpdateMyVehicleDetails(r.Context(), userID, vehicleID, service.VehicleDetailsInput{
		Name: req.Name, Axles: req.Axles, CapacityKg: req.CapacityKg, CapacityM3: req.CapacityM3,
		LengthM: req.LengthM, WidthM: req.WidthM, HeightM: req.HeightM, BodyType: req.BodyType,
	})
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, vehicle)
}

func (h *CargoHandler) UpdateMyVehicleName(w http.ResponseWriter, r *http.Request) {
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
	var req vehicleNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	vehicle, err := h.svc.UpdateMyVehicleName(r.Context(), userID, vehicleID, req.Name)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, vehicle)
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

type vehicleRegistrationRequest struct {
	RegistrationCountry string `json:"registration_country"`
	PlateNumber         string `json:"plate_number"`
	VIN                 string `json:"vin"`
	PrivacyConsent      bool   `json:"privacy_consent"`
}

func (h *CargoHandler) UpdateMyVehicleRegistration(w http.ResponseWriter, r *http.Request) {
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
	var req vehicleRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	vehicle, err := h.svc.UpdateMyVehicleRegistration(r.Context(), userID, vehicleID, req.RegistrationCountry, req.PlateNumber, req.VIN, req.PrivacyConsent)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, vehicle)
}

func (h *CargoHandler) UploadMyVehicleDocument(w http.ResponseWriter, r *http.Request) {
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
	const maxVehicleDocumentUploadSize = 12 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxVehicleDocumentUploadSize)
	if err := r.ParseMultipartForm(maxVehicleDocumentUploadSize); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "invalid multipart form")
		return
	}
	docType := models.VehicleDocumentType(r.FormValue("type"))
	_, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "file is required")
		return
	}
	vehicle, err := h.svc.UploadMyVehicleDocument(r.Context(), userID, vehicleID, docType, header)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, vehicle)
}

type vehicleTripRequest struct {
	Origin           geoPointPayload   `json:"origin"`
	Destination      geoPointPayload   `json:"destination"`
	Waypoints        []geoPointPayload `json:"waypoints"`
	CanPickupEnRoute bool              `json:"can_pickup_en_route"`
	DepartureDate    string            `json:"departure_date"`
	LoadedWeightKg   float64           `json:"loaded_weight_kg"`
	LoadedVolumeM3   float64           `json:"loaded_volume_m3"`
	Status           string            `json:"status"`
}

func parseVehicleTripRequest(req vehicleTripRequest) (service.VehicleTripInput, error) {
	departureDate, err := time.Parse("2006-01-02", req.DepartureDate)
	if err != nil {
		return service.VehicleTripInput{}, err
	}
	return service.VehicleTripInput{
		Origin: req.Origin.toModel(), Destination: req.Destination.toModel(), DepartureDate: departureDate,
		Waypoints: geoPayloadsToModels(req.Waypoints), CanPickupEnRoute: req.CanPickupEnRoute,
		LoadedWeightKg: req.LoadedWeightKg, LoadedVolumeM3: req.LoadedVolumeM3,
		Status: models.VehicleTripStatus(req.Status),
	}, nil
}

func (h *CargoHandler) AddMyVehicleTrip(w http.ResponseWriter, r *http.Request) {
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
	var req vehicleTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	input, err := parseVehicleTripRequest(req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", "departure_date must be YYYY-MM-DD")
		return
	}
	trip, err := h.svc.AddMyVehicleTrip(r.Context(), userID, vehicleID, input)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, trip)
}

func (h *CargoHandler) UpdateMyVehicleTrip(w http.ResponseWriter, r *http.Request) {
	vehicleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid vehicle id")
		return
	}
	tripID, err := uuid.Parse(chi.URLParam(r, "tid"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid trip id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	var req vehicleTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	input, err := parseVehicleTripRequest(req)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", "departure_date must be YYYY-MM-DD")
		return
	}
	trip, err := h.svc.UpdateMyVehicleTrip(r.Context(), userID, vehicleID, tripID, input)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, trip)
}

func (h *CargoHandler) DeleteMyVehicleTrip(w http.ResponseWriter, r *http.Request) {
	vehicleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid vehicle id")
		return
	}
	tripID, err := uuid.Parse(chi.URLParam(r, "tid"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid trip id")
		return
	}
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	if err := h.svc.DeleteMyVehicleTrip(r.Context(), userID, vehicleID, tripID); err != nil {
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
