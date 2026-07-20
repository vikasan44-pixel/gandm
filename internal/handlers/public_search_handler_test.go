package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestGeoPointFromQueryValidation(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantNil bool
		wantErr bool
	}{
		{name: "absent", wantNil: true},
		{name: "valid", query: "from_lat=43.2&from_lng=76.9"},
		{name: "missing longitude", query: "from_lat=43.2", wantErr: true},
		{name: "not numeric", query: "from_lat=x&from_lng=76.9", wantErr: true},
		{name: "outside WGS84", query: "from_lat=91&from_lng=76.9", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/?"+tc.query, nil)
			point, err := geoPointFromQuery(req, "from")
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantNil && point != nil {
				t.Fatalf("point = %#v, want nil", point)
			}
		})
	}
}

func TestVehicleSearchFilterRejectsInvalidNumbers(t *testing.T) {
	for _, query := range []string{
		"min_capacity_kg=-1",
		"min_capacity_m3=lots",
		"min_axles=-2",
		"from_lat=43.2",
	} {
		req := httptest.NewRequest("GET", "/?"+query, nil)
		if _, err := vehicleSearchFilterFromRequest(req); err == nil {
			t.Errorf("query %q accepted, want validation error", query)
		}
	}
}

func TestVehicleSearchFilterParsesValidValues(t *testing.T) {
	req := httptest.NewRequest("GET", "/?body_type=tent&min_capacity_kg=1500&min_axles=2&to_lat=43.8&to_lng=87.6", nil)
	filter, err := vehicleSearchFilterFromRequest(req)
	if err != nil {
		t.Fatalf("parse filter: %v", err)
	}
	if filter.BodyType != "tent" || filter.MinCapacityKg != 1500 || filter.MinAxles != 2 || filter.To == nil {
		t.Fatalf("unexpected filter: %#v", filter)
	}
}
