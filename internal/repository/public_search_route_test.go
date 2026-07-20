package repository

import (
	"testing"

	"gandm/internal/models"
)

func routePoint(lat, lng float64, label string) models.GeoPoint {
	return models.GeoPoint{Lat: lat, Lng: lng, Label: label, Country: "kz", Source: models.CoordSourceOSM}
}

func TestVehicleMatchesDirectionUsesOrderedWaypoints(t *testing.T) {
	vehicle := models.Vehicle{Trips: []models.VehicleTrip{{
		Origin: routePoint(43.24, 76.95, "Алматы"),
		Waypoints: []models.GeoPoint{
			routePoint(46.85, 74.98, "Балхаш"),
			routePoint(49.80, 73.10, "Караганда"),
		},
		Destination:      routePoint(51.17, 71.43, "Астана"),
		CanPickupEnRoute: true,
		Status:           models.VehicleTripPlanned,
	}}}
	balhash := routePoint(46.86, 75.00, "возле Балхаша")
	karaganda := routePoint(49.81, 73.12, "возле Караганды")
	if !vehicleMatchesDirection(vehicle, &balhash, &karaganda, 100, 50) {
		t.Fatal("cargo moving forward along the waypoint order must match")
	}
	if vehicleMatchesDirection(vehicle, &karaganda, &balhash, 100, 50) {
		t.Fatal("cargo moving backwards against the waypoint order must not match")
	}
}

func TestVehicleMatchesDirectionRequiresPickupFlagForIntermediateCities(t *testing.T) {
	vehicle := models.Vehicle{Trips: []models.VehicleTrip{{
		Origin:      routePoint(43.24, 76.95, "Алматы"),
		Waypoints:   []models.GeoPoint{routePoint(49.80, 73.10, "Караганда")},
		Destination: routePoint(51.17, 71.43, "Астана"),
		Status:      models.VehicleTripPlanned,
	}}}
	karaganda := routePoint(49.81, 73.12, "возле Караганды")
	astana := routePoint(51.18, 71.44, "возле Астаны")
	if vehicleMatchesDirection(vehicle, &karaganda, &astana, 100, 50) {
		t.Fatal("intermediate city must not match when en-route pickup is disabled")
	}
}

func TestSearchableVehicleTripsAllowsSeveralPlansUntilCargoIsActive(t *testing.T) {
	plans := []models.VehicleTrip{
		{Origin: routePoint(43.24, 76.95, "Алматы"), Destination: routePoint(51.17, 71.43, "Астана"), Status: models.VehicleTripPlanned},
		{Origin: routePoint(43.24, 76.95, "Алматы"), Destination: routePoint(42.32, 69.59, "Шымкент"), Status: models.VehicleTripPlanned},
	}
	if got := searchableVehicleTrips(plans); len(got) != 2 {
		t.Fatalf("searchable preliminary plans = %d, want 2", len(got))
	}

	plans[1].LoadedVolumeM3 = 12
	got := searchableVehicleTrips(plans)
	if len(got) != 1 || got[0].Destination.Label != "Шымкент" {
		t.Fatalf("active cargo must hide other plans, got %#v", got)
	}
}
