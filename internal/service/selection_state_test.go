package service

import (
	"testing"

	"github.com/google/uuid"

	"gandm/internal/models"
)

func TestGroupSelectionState(t *testing.T) {
	offerA := uuid.New()
	offerB := uuid.New()
	offerC := uuid.New()
	c1, c2, c3, c4, c5 := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()

	sel := func(client, offer uuid.UUID) models.ConsolidatedSelection {
		return models.ConsolidatedSelection{ClientID: client, OfferID: offer}
	}

	tests := []struct {
		name       string
		selections []models.ConsolidatedSelection
		accepted   []uuid.UUID
		wantState  string
		wantOffer  *uuid.UUID
	}{
		{"nobody chose", nil, []uuid.UUID{c1, c2}, "none", nil},
		{"pair: only one chose", []models.ConsolidatedSelection{sel(c1, offerA)}, []uuid.UUID{c1, c2}, "waiting_other", nil},
		{"pair: both same offer", []models.ConsolidatedSelection{sel(c1, offerA), sel(c2, offerA)}, []uuid.UUID{c1, c2}, "matched", &offerA},
		{"pair: different offers", []models.ConsolidatedSelection{sel(c1, offerA), sel(c2, offerB)}, []uuid.UUID{c1, c2}, "mismatch", nil},
		{"group of 5: majority of 3 closes the deal",
			[]models.ConsolidatedSelection{sel(c1, offerA), sel(c2, offerA), sel(c3, offerA), sel(c4, offerB)},
			[]uuid.UUID{c1, c2, c3, c4, c5}, "matched", &offerA},
		{"group of 5: 2-2-1 split, all voted — mismatch",
			[]models.ConsolidatedSelection{sel(c1, offerA), sel(c2, offerA), sel(c3, offerB), sel(c4, offerB), sel(c5, offerC)},
			[]uuid.UUID{c1, c2, c3, c4, c5}, "mismatch", nil},
		{"group of 3: 2 voted same, one silent — majority already",
			[]models.ConsolidatedSelection{sel(c1, offerA), sel(c2, offerA)},
			[]uuid.UUID{c1, c2, c3}, "matched", &offerA},
		{"non-accepted member's vote is ignored",
			[]models.ConsolidatedSelection{sel(c1, offerA), sel(c3, offerA)},
			[]uuid.UUID{c1, c2}, "waiting_other", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, offer := groupSelectionState(tt.selections, tt.accepted)
			if state != tt.wantState {
				t.Errorf("state = %q, want %q", state, tt.wantState)
			}
			switch {
			case tt.wantOffer == nil && offer != nil:
				t.Errorf("offer = %v, want nil", *offer)
			case tt.wantOffer != nil && (offer == nil || *offer != *tt.wantOffer):
				t.Errorf("offer = %v, want %v", offer, *tt.wantOffer)
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
