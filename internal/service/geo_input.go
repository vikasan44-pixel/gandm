package service

import (
	"fmt"
	"strings"

	"gandm/internal/geo"
	"gandm/internal/models"
)

// validateGeoPoint sanity-checks a point coming from the API. Coordinates
// are expected to already be WGS-84 — the frontend converts GCJ-02 (Amap)
// before submitting; `source` is provenance metadata, not a conversion flag.
func validateGeoPoint(field string, p models.GeoPoint) (models.GeoPoint, error) {
	p.Label = strings.TrimSpace(p.Label)
	if p.Label == "" {
		return p, fmt.Errorf("%w: %s.label is required", ErrInvalidInput, field)
	}
	if !geo.ValidLatLng(p.Lat, p.Lng) {
		return p, fmt.Errorf("%w: %s coordinates out of WGS-84 range", ErrInvalidInput, field)
	}
	if p.Source != models.CoordSourceAmap && p.Source != models.CoordSourceOSM {
		return p, fmt.Errorf("%w: %s.source must be \"amap\" or \"osm\"", ErrInvalidInput, field)
	}
	// Country comes from the geocoder; unknown (empty) is allowed and falls
	// into the default radius bucket. Normalized to lowercase ISO alpha-2.
	p.Country = strings.ToLower(strings.TrimSpace(p.Country))
	if len(p.Country) > 2 {
		return p, fmt.Errorf("%w: %s.country must be an ISO alpha-2 code or empty", ErrInvalidInput, field)
	}
	return p, nil
}
