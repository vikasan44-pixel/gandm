package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"gandm/internal/auth"
	"gandm/internal/geo"
	"gandm/internal/httpx"
	"gandm/internal/models"
	"gandm/internal/repository"
)

// Гостевой поиск (без авторизации). Точки передаются координатами из
// геокодера — тем же способом, что и везде на платформе, без текстового
// сопоставления городов.

// geoPointFromQuery собирает *GeoPoint из пары параметров <prefix>_lat/<prefix>_lng
// (+ опционально <prefix>_country/<prefix>_label). Возвращает nil, если обе
// координаты отсутствуют, и ошибку для неполной, нечисловой или выходящей за
// диапазон пары — ошибочный фильтр нельзя молча превращать в широкую выдачу.
func geoPointFromQuery(r *http.Request, prefix string) (*models.GeoPoint, error) {
	q := r.URL.Query()
	latRaw, lngRaw := strings.TrimSpace(q.Get(prefix+"_lat")), strings.TrimSpace(q.Get(prefix+"_lng"))
	if latRaw == "" && lngRaw == "" {
		return nil, nil
	}
	lat, errLat := strconv.ParseFloat(latRaw, 64)
	lng, errLng := strconv.ParseFloat(lngRaw, 64)
	if errLat != nil || errLng != nil || !geo.ValidLatLng(lat, lng) {
		return nil, fmt.Errorf("invalid %s coordinates", prefix)
	}
	return &models.GeoPoint{
		Lat:     lat,
		Lng:     lng,
		Label:   q.Get(prefix + "_label"),
		Country: q.Get(prefix + "_country"),
	}, nil
}

func floatQuery(r *http.Request, key string) (float64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return v, nil
}

// PublicSearchCargo — GET /api/public/cargo?from_lat=&from_lng=&to_lat=&to_lng=…
func (h *CargoHandler) PublicSearchCargo(w http.ResponseWriter, r *http.Request) {
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
	cards, err := h.svc.PublicSearchCargo(r.Context(), from, to)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cards)
}

// PublicSearchTransport — GET /api/public/transport?body_type=&min_capacity_kg=&from_lat=…
func vehicleSearchFilterFromRequest(r *http.Request) (repository.VehicleSearchFilter, error) {
	q := r.URL.Query()
	axles := 0
	if raw := strings.TrimSpace(q.Get("min_axles")); raw != "" {
		var err error
		axles, err = strconv.Atoi(raw)
		if err != nil || axles < 0 {
			return repository.VehicleSearchFilter{}, fmt.Errorf("invalid min_axles")
		}
	}
	capacityKg, err := floatQuery(r, "min_capacity_kg")
	if err != nil {
		return repository.VehicleSearchFilter{}, err
	}
	capacityM3, err := floatQuery(r, "min_capacity_m3")
	if err != nil {
		return repository.VehicleSearchFilter{}, err
	}
	length, err := floatQuery(r, "min_length_m")
	if err != nil {
		return repository.VehicleSearchFilter{}, err
	}
	width, err := floatQuery(r, "min_width_m")
	if err != nil {
		return repository.VehicleSearchFilter{}, err
	}
	height, err := floatQuery(r, "min_height_m")
	if err != nil {
		return repository.VehicleSearchFilter{}, err
	}
	from, err := geoPointFromQuery(r, "from")
	if err != nil {
		return repository.VehicleSearchFilter{}, err
	}
	to, err := geoPointFromQuery(r, "to")
	if err != nil {
		return repository.VehicleSearchFilter{}, err
	}
	return repository.VehicleSearchFilter{
		BodyType:      q.Get("body_type"),
		MinCapacityKg: capacityKg,
		MinCapacityM3: capacityM3,
		MinLengthM:    length,
		MinWidthM:     width,
		MinHeightM:    height,
		MinAxles:      axles,
		From:          from,
		To:            to,
	}, nil
}

func (h *CargoHandler) PublicSearchTransport(w http.ResponseWriter, r *http.Request) {
	filter, err := vehicleSearchFilterFromRequest(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	cards, err := h.svc.PublicSearchVehicles(r.Context(), filter)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cards)
}

// SearchAvailableTransport is the authenticated marketplace search. Unlike
// the guest endpoint, it excludes vehicles owned by the current user.
func (h *CargoHandler) SearchAvailableTransport(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	filter, err := vehicleSearchFilterFromRequest(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	cards, err := h.svc.SearchAvailableVehicles(r.Context(), userID, filter)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cards)
}
