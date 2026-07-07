package models

// CoordSource records which map the point came from — debugging metadata
// only. Coordinates are ALWAYS WGS-84 by the time they reach the API:
// GCJ-02 (Amap) input is converted on the frontend before submission.
type CoordSource string

const (
	CoordSourceAmap CoordSource = "amap"
	CoordSourceOSM  CoordSource = "osm"
)

// GeoPoint is a WGS-84 point with a human-readable label (the address the
// user picked). Matching compares coordinates by haversine distance with a
// per-country radius; the label is display-only.
//
// Country is a lowercase ISO-3166 alpha-2 code ("cn", "kz", …) filled by
// the frontend from the geocoder — empty string means unknown, which
// matching treats as the default (non-China) radius.
type GeoPoint struct {
	Lat     float64     `json:"lat"`
	Lng     float64     `json:"lng"`
	Label   string      `json:"label"`
	Source  CoordSource `json:"source"`
	Country string      `json:"country"`
}
