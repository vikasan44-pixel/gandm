package service

import (
	"testing"

	"github.com/google/uuid"

	"gandm/internal/models"
)

func TestSelectionState(t *testing.T) {
	s := &CargoService{}
	offerA := uuid.New()
	offerB := uuid.New()

	tests := []struct {
		name        string
		mine, other *uuid.UUID
		want        string
	}{
		{"nobody chose", nil, nil, "none"},
		{"only mine", &offerA, nil, "waiting_other"},
		{"only other", nil, &offerA, "waiting_other"},
		{"same offer", &offerA, &offerA, "matched"},
		{"different offers", &offerA, &offerB, "mismatch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.selectionState(tt.mine, tt.other); got != tt.want {
				t.Errorf("selectionState() = %q, want %q", got, tt.want)
			}
		})
	}
}

func validPoint() models.GeoPoint {
	return models.GeoPoint{Lat: 43.25, Lng: 76.9, Label: "Алматы", Source: models.CoordSourceOSM, Country: "KZ"}
}

func TestValidateGeoPoint(t *testing.T) {
	t.Run("normalizes country to lowercase", func(t *testing.T) {
		p, err := validateGeoPoint("origin", validPoint())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Country != "kz" {
			t.Errorf("Country = %q, want %q", p.Country, "kz")
		}
	})

	t.Run("rejects empty label", func(t *testing.T) {
		in := validPoint()
		in.Label = "   "
		if _, err := validateGeoPoint("origin", in); err == nil {
			t.Error("expected error for blank label")
		}
	})

	t.Run("rejects out-of-range coordinates", func(t *testing.T) {
		in := validPoint()
		in.Lat = 91
		if _, err := validateGeoPoint("origin", in); err == nil {
			t.Error("expected error for lat 91")
		}
	})

	t.Run("rejects unknown source", func(t *testing.T) {
		in := validPoint()
		in.Source = "google"
		if _, err := validateGeoPoint("origin", in); err == nil {
			t.Error("expected error for unknown source")
		}
	})

	t.Run("rejects long country code", func(t *testing.T) {
		in := validPoint()
		in.Country = "kaz"
		if _, err := validateGeoPoint("origin", in); err == nil {
			t.Error("expected error for 3-letter country")
		}
	})

	t.Run("allows empty country", func(t *testing.T) {
		in := validPoint()
		in.Country = ""
		if _, err := validateGeoPoint("origin", in); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
