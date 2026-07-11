package handlers

import (
	"net/http"
	"strconv"

	"gandm/internal/httpx"
	"gandm/internal/models"
	"gandm/internal/repository"
)

// Гостевой поиск (без авторизации). Точки передаются координатами из
// геокодера — тем же способом, что и везде на платформе, без текстового
// сопоставления городов.

// geoPointFromQuery собирает *GeoPoint из пары параметров <prefix>_lat/<prefix>_lng
// (+ опционально <prefix>_country/<prefix>_label). Возвращает nil, если координат
// нет или они не парсятся — то есть фильтр по этой точке просто не применяется.
func geoPointFromQuery(r *http.Request, prefix string) *models.GeoPoint {
	q := r.URL.Query()
	lat, errLat := strconv.ParseFloat(q.Get(prefix+"_lat"), 64)
	lng, errLng := strconv.ParseFloat(q.Get(prefix+"_lng"), 64)
	if errLat != nil || errLng != nil {
		return nil
	}
	return &models.GeoPoint{
		Lat:     lat,
		Lng:     lng,
		Label:   q.Get(prefix + "_label"),
		Country: q.Get(prefix + "_country"),
	}
}

func floatQuery(r *http.Request, key string) float64 {
	v, _ := strconv.ParseFloat(r.URL.Query().Get(key), 64)
	return v
}

// PublicSearchCargo — GET /api/public/cargo?from_lat=&from_lng=&to_lat=&to_lng=…
func (h *CargoHandler) PublicSearchCargo(w http.ResponseWriter, r *http.Request) {
	from := geoPointFromQuery(r, "from")
	to := geoPointFromQuery(r, "to")
	cards, err := h.svc.PublicSearchCargo(r.Context(), from, to)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cards)
}

// PublicSearchTransport — GET /api/public/transport?body_type=&min_capacity_kg=&from_lat=…
func (h *CargoHandler) PublicSearchTransport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	axles, _ := strconv.Atoi(q.Get("min_axles"))
	filter := repository.VehicleSearchFilter{
		BodyType:      q.Get("body_type"),
		MinCapacityKg: floatQuery(r, "min_capacity_kg"),
		MinCapacityM3: floatQuery(r, "min_capacity_m3"),
		MinLengthM:    floatQuery(r, "min_length_m"),
		MinWidthM:     floatQuery(r, "min_width_m"),
		MinHeightM:    floatQuery(r, "min_height_m"),
		MinAxles:      axles,
		From:          geoPointFromQuery(r, "from"),
		To:            geoPointFromQuery(r, "to"),
	}
	cards, err := h.svc.PublicSearchVehicles(r.Context(), filter)
	if err != nil {
		writeCargoServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cards)
}
