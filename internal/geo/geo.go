// Package geo holds coordinate math for route matching. All coordinates in
// this codebase are WGS-84 — GCJ-02 (Amap) input is converted on the
// frontend before it ever reaches the API.
package geo

import "math"

const earthRadiusKm = 6371.0

// HaversineKm returns the great-circle distance between two WGS-84 points
// in kilometers.
func HaversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const rad = math.Pi / 180

	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)

	return 2 * earthRadiusKm * math.Asin(math.Sqrt(a))
}

// ValidLatLng reports whether the pair is a plausible WGS-84 coordinate.
func ValidLatLng(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}
