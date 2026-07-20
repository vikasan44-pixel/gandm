package service

import (
	"errors"
	"testing"
	"time"

	"gandm/internal/models"
)

func validVehicleInputForTest() VehicleInput {
	return VehicleInput{
		Name:  "Truck 1",
		Axles: 2, CapacityKg: 23000, CapacityM3: 90,
		LengthM: 13.6, WidthM: 2.45, HeightM: 4, BodyType: "bodyTented",
	}
}

func TestValidateVehicleTripInputUsesLocationRadiusAndAcceptsLegacySource(t *testing.T) {
	vehicle := &models.Vehicle{
		CapacityKg: 23000,
		CapacityM3: 90,
		Location:   &models.GeoPoint{Lat: 43.2363924, Lng: 76.9457275, Label: "Алматы", Country: "kz"},
	}
	input := VehicleTripInput{
		Origin:         models.GeoPoint{Lat: 43.24, Lng: 76.95, Label: "Алматы", Country: "kz"},
		Destination:    models.GeoPoint{Lat: 51.08584, Lng: 71.43311, Label: "Астана", Country: "kz", Source: models.CoordSourceOSM},
		DepartureDate:  time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
		LoadedVolumeM3: 90,
		Status:         models.VehicleTripLoading,
	}
	valid, err := validateVehicleTripInput(vehicle, input, 50)
	if err != nil {
		t.Fatalf("nearby trip with legacy empty source must be accepted: %v", err)
	}
	if valid.Origin.Source != models.CoordSourceOSM {
		t.Fatalf("legacy source = %q, want osm", valid.Origin.Source)
	}

	input.Origin = models.GeoPoint{Lat: 44.0, Lng: 76.95, Label: "Too far", Country: "kz", Source: models.CoordSourceOSM}
	if _, err := validateVehicleTripInput(vehicle, input, 50); !errors.Is(err, ErrVehicleTripOriginTooFar) {
		t.Fatalf("far origin error = %v, want ErrVehicleTripOriginTooFar", err)
	}
	input.Status = models.VehicleTripCompleted
	if _, err := validateVehicleTripInput(vehicle, input, 50); err != nil {
		t.Fatalf("completed historical trip may be edited after the vehicle moved: %v", err)
	}
}

func TestValidateVehicleInputVerificationIsOptional(t *testing.T) {
	input := validVehicleInputForTest()
	if err := validateVehicleInput(input); err != nil {
		t.Fatalf("vehicle without verification must be accepted: %v", err)
	}

	input.RegistrationCountry = "KZ"
	if err := validateVehicleInput(input); err == nil {
		t.Fatal("partially started verification must require the complete identity and consent")
	}

	input.PlateNumber = "123ABC02"
	input.VIN = "1HGCM82633A004352"
	input.PrivacyConsent = true
	if err := validateVehicleInput(input); err != nil {
		t.Fatalf("complete verification identity must be accepted: %v", err)
	}
}
