package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"gandm/internal/models"
	"gandm/internal/repository"
)

// ToolManageFleet gates the vehicle CRUD (ТЗ §11.1) — possession of the
// tool, never participant_type, decides access (Block A principle).
const ToolManageFleet = "manage_fleet"

var ErrVehicleLimitReached = errors.New("vehicle limit reached for this account")

// defaultVehicleLimit mirrors the ТЗ rule «физлицо — максимум 2 машины».
// The schema has no legal-form field, so the limit applies to everyone and
// is adjustable platform-wide via the vehicle_limit_per_user setting (ИП/ТОО
// «без ограничений» → set it high). Flagged as an interpretation.
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
	Axles      int
	CapacityKg float64
	CapacityM3 float64
	LengthM    float64
	WidthM     float64
	HeightM    float64
	BodyType   string
	// Опциональное местонахождение координатами (по карте) — «откуда».
	Location *models.GeoPoint
	// Ноль или несколько назначений (координатами) — «куда».
	Destinations []models.GeoPoint
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

func validateVehicleInput(in VehicleInput) error {
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

	vehicleRepo := repository.NewVehicleRepository(s.db)
	count, err := vehicleRepo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if count >= s.vehicleLimit(ctx) {
		return nil, ErrVehicleLimitReached
	}

	vehicle := &models.Vehicle{
		ID:         uuid.New(),
		UserID:     userID,
		Axles:      in.Axles,
		CapacityKg: in.CapacityKg,
		CapacityM3: in.CapacityM3,
		LengthM:    in.LengthM,
		WidthM:     in.WidthM,
		HeightM:    in.HeightM,
		BodyType:   strings.TrimSpace(in.BodyType),
		Location:   location,
		CreatedAt:  time.Now(),
	}
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
