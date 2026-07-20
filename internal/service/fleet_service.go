package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"gandm/internal/geo"
	"gandm/internal/models"
	"gandm/internal/repository"
)

// ToolManageFleet gates the vehicle CRUD (ТЗ §11.1) — possession of the
// tool, never participant_type, decides access (Block A principle).
const ToolManageFleet = "manage_fleet"

const VehiclePrivacyConsentVersion = "vehicle-verification-v1"

var (
	ErrVehicleLimitReached     = errors.New("vehicle limit reached for this account")
	ErrVehicleTripOriginTooFar = errors.New("trip origin is too far from vehicle location")
)

// defaultVehicleLimit mirrors the rule «физлицо — максимум 2 машины».
// Legal entities are not capped by this participant-level limit.
const defaultVehicleLimit = 2

func (s *CargoService) vehicleLimit(ctx context.Context) int {
	settingsRepo := repository.NewSettingsRepository(s.db)
	raw, err := settingsRepo.Get(ctx, repository.SettingVehicleLimitPerUser)
	if err != nil {
		return defaultVehicleLimit
	}
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return defaultVehicleLimit
	}
	return limit
}

type VehicleInput struct {
	Name                string
	Axles               int
	CapacityKg          float64
	CapacityM3          float64
	LengthM             float64
	WidthM              float64
	HeightM             float64
	BodyType            string
	RegistrationCountry string
	PlateNumber         string
	VIN                 string
	PrivacyConsent      bool
	// Опциональное местонахождение координатами (по карте) — «откуда».
	Location *models.GeoPoint
	// Ноль или несколько назначений (координатами) — «куда».
	Destinations []models.GeoPoint
}

type VehicleDetailsInput struct {
	Name       string
	Axles      int
	CapacityKg float64
	CapacityM3 float64
	LengthM    float64
	WidthM     float64
	HeightM    float64
	BodyType   string
}

type VehicleTripInput struct {
	Origin           models.GeoPoint
	Destination      models.GeoPoint
	Waypoints        []models.GeoPoint
	CanPickupEnRoute bool
	DepartureDate    time.Time
	LoadedWeightKg   float64
	LoadedVolumeM3   float64
	Status           models.VehicleTripStatus
}

// validateVehicleGeo проверяет опциональное местонахождение и список
// назначений (каждое — координаты из геокодера). Возвращает нормализованные
// значения.
func validateVehicleGeo(in VehicleInput) (loc *models.GeoPoint, dests []models.GeoPoint, err error) {
	if in.Location != nil {
		l, e := validateGeoPoint("location", *in.Location)
		if e != nil {
			return nil, nil, e
		}
		loc = &l
	}
	for i, d := range in.Destinations {
		vd, e := validateGeoPoint(fmt.Sprintf("destinations[%d]", i), d)
		if e != nil {
			return nil, nil, e
		}
		dests = append(dests, vd)
	}
	return loc, dests, nil
}

func validateVehicleDetails(in VehicleDetailsInput) error {
	if len([]rune(strings.TrimSpace(in.Name))) > 80 {
		return fmt.Errorf("%w: vehicle name must not exceed 80 characters", ErrInvalidInput)
	}
	if in.Axles < 1 || in.Axles > 12 {
		return fmt.Errorf("%w: axles must be between 1 and 12", ErrInvalidInput)
	}
	if in.CapacityKg <= 0 {
		return fmt.Errorf("%w: capacity_kg must be positive", ErrInvalidInput)
	}
	if in.CapacityM3 < 0 {
		return fmt.Errorf("%w: capacity_m3 must not be negative", ErrInvalidInput)
	}
	if in.LengthM <= 0 || in.WidthM <= 0 || in.HeightM <= 0 {
		return fmt.Errorf("%w: dimensions must be positive", ErrInvalidInput)
	}
	if strings.TrimSpace(in.BodyType) == "" {
		return fmt.Errorf("%w: body_type is required", ErrInvalidInput)
	}
	return nil
}

func validateVehicleInput(in VehicleInput) error {
	if err := validateVehicleDetails(VehicleDetailsInput{
		Name: in.Name, Axles: in.Axles, CapacityKg: in.CapacityKg, CapacityM3: in.CapacityM3,
		LengthM: in.LengthM, WidthM: in.WidthM, HeightM: in.HeightM, BodyType: in.BodyType,
	}); err != nil {
		return err
	}
	if vehicleVerificationRequested(in) {
		if strings.TrimSpace(in.RegistrationCountry) == "" || strings.TrimSpace(in.PlateNumber) == "" {
			return fmt.Errorf("%w: registration country and plate number are required for verification", ErrInvalidInput)
		}
		if !validVIN(in.VIN) {
			return fmt.Errorf("%w: VIN must contain 17 valid characters", ErrInvalidInput)
		}
		if !in.PrivacyConsent {
			return fmt.Errorf("%w: privacy consent is required for verification", ErrInvalidInput)
		}
	}
	return nil
}

func (s *CargoService) UpdateMyVehicleDetails(ctx context.Context, userID, vehicleID uuid.UUID, in VehicleDetailsInput) (*models.Vehicle, error) {
	vehicleRepo, vehicle, err := s.ownedVehicle(ctx, userID, vehicleID)
	if err != nil {
		return nil, err
	}
	if err := validateVehicleDetails(in); err != nil {
		return nil, err
	}
	for _, trip := range vehicle.Trips {
		if trip.LoadedWeightKg > in.CapacityKg || trip.LoadedVolumeM3 > in.CapacityM3 {
			return nil, fmt.Errorf("%w: capacity cannot be lower than an existing trip load", ErrInvalidInput)
		}
	}
	vehicle.Name = strings.TrimSpace(in.Name)
	vehicle.Axles = in.Axles
	vehicle.CapacityKg = in.CapacityKg
	vehicle.CapacityM3 = in.CapacityM3
	vehicle.LengthM = in.LengthM
	vehicle.WidthM = in.WidthM
	vehicle.HeightM = in.HeightM
	vehicle.BodyType = strings.TrimSpace(in.BodyType)
	if err := vehicleRepo.UpdateDetails(ctx, vehicle); err != nil {
		return nil, err
	}
	return vehicle, nil
}

func vehicleVerificationRequested(in VehicleInput) bool {
	return strings.TrimSpace(in.RegistrationCountry) != "" ||
		strings.TrimSpace(in.PlateNumber) != "" ||
		strings.TrimSpace(in.VIN) != "" || in.PrivacyConsent
}

func validVIN(value string) bool {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != 17 {
		return false
	}
	for _, char := range value {
		if !((char >= 'A' && char <= 'Z' && char != 'I' && char != 'O' && char != 'Q') || (char >= '0' && char <= '9')) {
			return false
		}
	}
	return true
}

func (s *CargoService) vehicleTripOriginRadius(location *models.GeoPoint, origin models.GeoPoint) float64 {
	if (location != nil && strings.EqualFold(location.Country, "cn")) || strings.EqualFold(origin.Country, "cn") {
		return s.cfg.MatchRadiusCNKm
	}
	return s.cfg.MatchRadiusKZKm
}

func (s *CargoService) ListMyVehicles(ctx context.Context, userID uuid.UUID) ([]models.Vehicle, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, userID, ToolManageFleet); err != nil {
		return nil, err
	}
	return repository.NewVehicleRepository(s.db).ListByUserID(ctx, userID)
}

func (s *CargoService) AddMyVehicle(ctx context.Context, userID uuid.UUID, in VehicleInput) (*models.Vehicle, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, userID, ToolManageFleet); err != nil {
		return nil, err
	}
	if err := validateVehicleInput(in); err != nil {
		return nil, err
	}
	location, dests, err := validateVehicleGeo(in)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// The user row is the quota lock: concurrent vehicle creations for the
	// same individual cannot both pass the count check.
	user, err := repository.NewUserRepository(tx).GetByIDForUpdate(ctx, userID)
	if err != nil {
		return nil, err
	}
	vehicleRepo := repository.NewVehicleRepository(tx)
	if user.LegalForm == models.LegalFormIndividual {
		count, err := vehicleRepo.CountByUserID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if count >= s.vehicleLimit(ctx) {
			return nil, ErrVehicleLimitReached
		}
	}

	verificationRequested := vehicleVerificationRequested(in)
	vehicle := &models.Vehicle{
		ID:                  uuid.New(),
		UserID:              userID,
		Name:                strings.TrimSpace(in.Name),
		Axles:               in.Axles,
		CapacityKg:          in.CapacityKg,
		CapacityM3:          in.CapacityM3,
		LengthM:             in.LengthM,
		WidthM:              in.WidthM,
		HeightM:             in.HeightM,
		BodyType:            strings.TrimSpace(in.BodyType),
		RegistrationCountry: strings.ToUpper(strings.TrimSpace(in.RegistrationCountry)),
		PlateNumber:         strings.ToUpper(strings.Join(strings.Fields(in.PlateNumber), "")),
		VIN:                 strings.ToUpper(strings.TrimSpace(in.VIN)),
		VerificationStatus:  models.VehicleVerificationNotSubmitted,
		Location:            location,
		CreatedAt:           time.Now(),
	}
	if verificationRequested {
		vehicle.PrivacyConsentAt = &vehicle.CreatedAt
		vehicle.PrivacyConsentVersion = VehiclePrivacyConsentVersion
	}
	vehicle.TrustPercent = 50
	vehicle.UploadedDocumentTypes = []models.VehicleDocumentType{}
	if err := vehicleRepo.Create(ctx, vehicle); err != nil {
		return nil, err
	}
	// Начальные назначения (могут отсутствовать — добавит позже).
	vehicle.Destinations = make([]models.VehicleDestination, 0, len(dests))
	for _, d := range dests {
		saved, err := vehicleRepo.AddDestination(ctx, vehicle.ID, d)
		if err != nil {
			return nil, err
		}
		vehicle.Destinations = append(vehicle.Destinations, saved)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return vehicle, nil
}

// UpdateMyVehicleName changes the owner's private fleet label. The name is
// used only in the cabinet to distinguish similar vehicles.
func (s *CargoService) UpdateMyVehicleName(ctx context.Context, userID, vehicleID uuid.UUID, name string) (*models.Vehicle, error) {
	vehicleRepo, vehicle, err := s.ownedVehicle(ctx, userID, vehicleID)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return nil, fmt.Errorf("%w: vehicle name must contain 1 to 80 characters", ErrInvalidInput)
	}
	if err := vehicleRepo.UpdateName(ctx, vehicleID, name); err != nil {
		return nil, err
	}
	vehicle.Name = name
	return vehicle, nil
}

// UpdateMyVehicleLocation — «местонахождение можно обновлять в любое время»
// (ТЗ §11.1). Теперь координатами (по карте); nil очищает точку. Чужая машина
// читается как not-found (та же политика, что у маршрутов).
func (s *CargoService) UpdateMyVehicleLocation(ctx context.Context, userID, vehicleID uuid.UUID, location *models.GeoPoint) (*models.Vehicle, error) {
	vehicleRepo, vehicle, err := s.ownedVehicle(ctx, userID, vehicleID)
	if err != nil {
		return nil, err
	}
	var loc *models.GeoPoint
	if location != nil {
		l, e := validateGeoPoint("location", *location)
		if e != nil {
			return nil, e
		}
		loc = &l
	}
	if err := vehicleRepo.UpdateLocation(ctx, vehicleID, loc); err != nil {
		return nil, err
	}
	vehicle.Location = loc
	return vehicle, nil
}

// AddMyVehicleDestination adds one more destination to the owner's vehicle.
func (s *CargoService) AddMyVehicleDestination(ctx context.Context, userID, vehicleID uuid.UUID, point models.GeoPoint) (*models.VehicleDestination, error) {
	vehicleRepo, _, err := s.ownedVehicle(ctx, userID, vehicleID)
	if err != nil {
		return nil, err
	}
	p, err := validateGeoPoint("destination", point)
	if err != nil {
		return nil, err
	}
	saved, err := vehicleRepo.AddDestination(ctx, vehicleID, p)
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

// DeleteMyVehicleDestination removes one destination from the owner's vehicle.
func (s *CargoService) DeleteMyVehicleDestination(ctx context.Context, userID, vehicleID, destID uuid.UUID) error {
	vehicleRepo, _, err := s.ownedVehicle(ctx, userID, vehicleID)
	if err != nil {
		return err
	}
	return vehicleRepo.DeleteDestination(ctx, vehicleID, destID)
}

func validateVehicleTripInput(vehicle *models.Vehicle, input VehicleTripInput, originRadiusKm float64) (VehicleTripInput, error) {
	origin, err := validateGeoPoint("origin", input.Origin)
	if err != nil {
		return input, err
	}
	destination, err := validateGeoPoint("destination", input.Destination)
	if err != nil {
		return input, err
	}
	if input.DepartureDate.IsZero() {
		return input, fmt.Errorf("%w: departure_date is required", ErrInvalidInput)
	}
	if len(input.Waypoints) > 12 {
		return input, fmt.Errorf("%w: no more than 12 route cities are allowed", ErrInvalidInput)
	}
	waypoints := make([]models.GeoPoint, 0, len(input.Waypoints))
	if input.CanPickupEnRoute {
		for index, waypoint := range input.Waypoints {
			validated, err := validateGeoPoint(fmt.Sprintf("waypoints[%d]", index), waypoint)
			if err != nil {
				return input, err
			}
			waypoints = append(waypoints, validated)
		}
	}
	if input.Status != models.VehicleTripCompleted && vehicle.Location != nil && geo.HaversineKm(vehicle.Location.Lat, vehicle.Location.Lng, origin.Lat, origin.Lng) > originRadiusKm {
		return input, fmt.Errorf("%w: origin must be within %.0f km of vehicle location", ErrVehicleTripOriginTooFar, originRadiusKm)
	}
	if input.LoadedWeightKg < 0 || input.LoadedVolumeM3 < 0 {
		return input, fmt.Errorf("%w: loaded values must not be negative", ErrInvalidInput)
	}
	if input.LoadedWeightKg > vehicle.CapacityKg {
		return input, fmt.Errorf("%w: loaded_weight_kg exceeds vehicle capacity", ErrInvalidInput)
	}
	if input.LoadedVolumeM3 > vehicle.CapacityM3 {
		return input, fmt.Errorf("%w: loaded_volume_m3 exceeds vehicle capacity", ErrInvalidInput)
	}
	if input.Status == "" {
		input.Status = models.VehicleTripPlanned
	}
	switch input.Status {
	case models.VehicleTripPlanned, models.VehicleTripLoading, models.VehicleTripDeparted, models.VehicleTripCompleted:
	default:
		return input, fmt.Errorf("%w: invalid trip status", ErrInvalidInput)
	}
	input.Origin = origin
	input.Destination = destination
	input.Waypoints = waypoints
	return input, nil
}

func (s *CargoService) AddMyVehicleTrip(ctx context.Context, userID, vehicleID uuid.UUID, input VehicleTripInput) (*models.VehicleTrip, error) {
	vehicleRepo, vehicle, err := s.ownedVehicle(ctx, userID, vehicleID)
	if err != nil {
		return nil, err
	}
	valid, err := validateVehicleTripInput(vehicle, input, s.vehicleTripOriginRadius(vehicle.Location, input.Origin))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	trip := &models.VehicleTrip{
		ID: uuid.New(), VehicleID: vehicleID, Origin: valid.Origin, Destination: valid.Destination,
		Waypoints: valid.Waypoints, CanPickupEnRoute: valid.CanPickupEnRoute,
		DepartureDate: valid.DepartureDate, LoadedWeightKg: valid.LoadedWeightKg,
		LoadedVolumeM3: valid.LoadedVolumeM3, Status: valid.Status, CreatedAt: now, UpdatedAt: now,
	}
	if err := vehicleRepo.CreateTrip(ctx, trip); err != nil {
		return nil, err
	}
	return trip, nil
}

func (s *CargoService) UpdateMyVehicleTrip(ctx context.Context, userID, vehicleID, tripID uuid.UUID, input VehicleTripInput) (*models.VehicleTrip, error) {
	_, vehicle, err := s.ownedVehicle(ctx, userID, vehicleID)
	if err != nil {
		return nil, err
	}
	valid, err := validateVehicleTripInput(vehicle, input, s.vehicleTripOriginRadius(vehicle.Location, input.Origin))
	if err != nil {
		return nil, err
	}
	trip := &models.VehicleTrip{
		ID: tripID, VehicleID: vehicleID, Origin: valid.Origin, Destination: valid.Destination,
		Waypoints: valid.Waypoints, CanPickupEnRoute: valid.CanPickupEnRoute,
		DepartureDate: valid.DepartureDate, LoadedWeightKg: valid.LoadedWeightKg,
		LoadedVolumeM3: valid.LoadedVolumeM3, Status: valid.Status, UpdatedAt: time.Now(),
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	txVehicleRepo := repository.NewVehicleRepository(tx)
	if err := txVehicleRepo.UpdateTrip(ctx, trip); err != nil {
		return nil, err
	}
	if trip.Status == models.VehicleTripCompleted {
		if err := txVehicleRepo.UpdateLocation(ctx, vehicleID, &trip.Destination); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return trip, nil
}

func (s *CargoService) DeleteMyVehicleTrip(ctx context.Context, userID, vehicleID, tripID uuid.UUID) error {
	vehicleRepo, _, err := s.ownedVehicle(ctx, userID, vehicleID)
	if err != nil {
		return err
	}
	return vehicleRepo.DeleteTrip(ctx, vehicleID, tripID)
}

// ownedVehicle loads a vehicle after checking the user is active, holds the
// fleet tool, and owns it (else not-found). Shared by the mutating methods.
func (s *CargoService) ownedVehicle(ctx context.Context, userID, vehicleID uuid.UUID) (*repository.VehicleRepository, *models.Vehicle, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, nil, err
	}
	if err := s.requireTool(ctx, userID, ToolManageFleet); err != nil {
		return nil, nil, err
	}
	vehicleRepo := repository.NewVehicleRepository(s.db)
	vehicle, err := vehicleRepo.GetByID(ctx, vehicleID)
	if err != nil {
		return nil, nil, err
	}
	if vehicle.UserID != userID {
		return nil, nil, repository.ErrNotFound
	}
	return vehicleRepo, vehicle, nil
}

func (s *CargoService) DeleteMyVehicle(ctx context.Context, userID, vehicleID uuid.UUID) error {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return err
	}
	if err := s.requireTool(ctx, userID, ToolManageFleet); err != nil {
		return err
	}

	vehicleRepo := repository.NewVehicleRepository(s.db)
	vehicle, err := vehicleRepo.GetByID(ctx, vehicleID)
	if err != nil {
		return err
	}
	if vehicle.UserID != userID {
		return repository.ErrNotFound
	}
	return vehicleRepo.Delete(ctx, vehicleID)
}
