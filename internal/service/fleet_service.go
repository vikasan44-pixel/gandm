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
	Axles           int
	CapacityKg      float64
	LengthM         float64
	WidthM          float64
	HeightM         float64
	BodyType        string
	CurrentLocation string
}

func validateVehicleInput(in VehicleInput) error {
	if in.Axles < 1 || in.Axles > 12 {
		return fmt.Errorf("%w: axles must be between 1 and 12", ErrInvalidInput)
	}
	if in.CapacityKg <= 0 {
		return fmt.Errorf("%w: capacity_kg must be positive", ErrInvalidInput)
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

	vehicleRepo := repository.NewVehicleRepository(s.db)
	count, err := vehicleRepo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if count >= s.vehicleLimit(ctx) {
		return nil, ErrVehicleLimitReached
	}

	vehicle := &models.Vehicle{
		ID:              uuid.New(),
		UserID:          userID,
		Axles:           in.Axles,
		CapacityKg:      in.CapacityKg,
		LengthM:         in.LengthM,
		WidthM:          in.WidthM,
		HeightM:         in.HeightM,
		BodyType:        strings.TrimSpace(in.BodyType),
		CurrentLocation: strings.TrimSpace(in.CurrentLocation),
		CreatedAt:       time.Now(),
	}
	if err := vehicleRepo.Create(ctx, vehicle); err != nil {
		return nil, err
	}
	return vehicle, nil
}

// UpdateMyVehicleLocation — «текущее местонахождение можно обновлять в любое
// время» (ТЗ §11.1). Someone else's vehicle reads as not-found, not
// forbidden (same policy as routes).
func (s *CargoService) UpdateMyVehicleLocation(ctx context.Context, userID, vehicleID uuid.UUID, location string) (*models.Vehicle, error) {
	if err := s.requireActiveUser(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.requireTool(ctx, userID, ToolManageFleet); err != nil {
		return nil, err
	}

	vehicleRepo := repository.NewVehicleRepository(s.db)
	vehicle, err := vehicleRepo.GetByID(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	if vehicle.UserID != userID {
		return nil, repository.ErrNotFound
	}
	vehicle.CurrentLocation = strings.TrimSpace(location)
	if err := vehicleRepo.UpdateLocation(ctx, vehicleID, vehicle.CurrentLocation); err != nil {
		return nil, err
	}
	return vehicle, nil
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
