package geo

import (
	"math"
	"testing"
)

func TestHaversineKm(t *testing.T) {
	tests := []struct {
		name                   string
		lat1, lng1, lat2, lng2 float64
		wantKm                 float64
		toleranceKm            float64
	}{
		{"same point", 43.25, 76.9, 43.25, 76.9, 0, 0.001},
		// One degree of longitude on the equator ≈ 111.19 km.
		{"one degree on equator", 0, 0, 0, 1, 111.19, 0.1},
		// Almaty — Khorgos, reference distance ≈ 306 km.
		{"almaty to khorgos", 43.25, 76.9, 44.21, 80.42, 306, 5},
		{"antipodes", 0, 0, 0, 180, math.Pi * 6371.0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HaversineKm(tt.lat1, tt.lng1, tt.lat2, tt.lng2)
			if math.Abs(got-tt.wantKm) > tt.toleranceKm {
				t.Errorf("HaversineKm() = %.3f km, want %.3f ± %.3f", got, tt.wantKm, tt.toleranceKm)
			}
		})
	}
}

func TestHaversineKmSymmetric(t *testing.T) {
	ab := HaversineKm(43.25, 76.9, 39.9, 116.4)
	ba := HaversineKm(39.9, 116.4, 43.25, 76.9)
	if math.Abs(ab-ba) > 1e-9 {
		t.Errorf("distance not symmetric: %v vs %v", ab, ba)
	}
}

func TestValidLatLng(t *testing.T) {
	tests := []struct {
		lat, lng float64
		want     bool
	}{
		{0, 0, true},
		{90, 180, true},
		{-90, -180, true},
		{90.01, 0, false},
		{-90.01, 0, false},
		{0, 180.01, false},
		{0, -180.01, false},
	}
	for _, tt := range tests {
		if got := ValidLatLng(tt.lat, tt.lng); got != tt.want {
			t.Errorf("ValidLatLng(%v, %v) = %v, want %v", tt.lat, tt.lng, got, tt.want)
		}
	}
}
