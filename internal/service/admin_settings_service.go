package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"gandm/internal/repository"
)

// PlatformSettings is the admin-editable runtime configuration (currently
// the consolidation capacity limits). Stored in the DB so changes apply
// without a restart.
type PlatformSettings struct {
	MaxVolumeM3 float64 `json:"max_volume_m3"`
	MaxWeightKg float64 `json:"max_weight_kg"`
}

func (s *AdminService) GetPlatformSettings(ctx context.Context) (*PlatformSettings, error) {
	settingsRepo := repository.NewSettingsRepository(s.db)

	out := &PlatformSettings{}
	rawVolume, err := settingsRepo.Get(ctx, repository.SettingMaxVolumeM3)
	if err != nil {
		return nil, err
	}
	out.MaxVolumeM3, err = strconv.ParseFloat(rawVolume, 64)
	if err != nil {
		return nil, fmt.Errorf("corrupted setting %s: %w", repository.SettingMaxVolumeM3, err)
	}

	rawWeight, err := settingsRepo.Get(ctx, repository.SettingMaxWeightKg)
	if err != nil {
		return nil, err
	}
	out.MaxWeightKg, err = strconv.ParseFloat(rawWeight, 64)
	if err != nil {
		return nil, fmt.Errorf("corrupted setting %s: %w", repository.SettingMaxWeightKg, err)
	}
	return out, nil
}

func (s *AdminService) UpdatePlatformSettings(ctx context.Context, adminID uuid.UUID, in PlatformSettings) (*PlatformSettings, error) {
	if in.MaxVolumeM3 <= 0 {
		return nil, fmt.Errorf("%w: max_volume_m3 must be positive", ErrInvalidInput)
	}
	if in.MaxWeightKg <= 0 {
		return nil, fmt.Errorf("%w: max_weight_kg must be positive", ErrInvalidInput)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	settingsRepo := repository.NewSettingsRepository(tx)
	if err := settingsRepo.Set(ctx, repository.SettingMaxVolumeM3, strconv.FormatFloat(in.MaxVolumeM3, 'f', -1, 64)); err != nil {
		return nil, err
	}
	if err := settingsRepo.Set(ctx, repository.SettingMaxWeightKg, strconv.FormatFloat(in.MaxWeightKg, 'f', -1, 64)); err != nil {
		return nil, err
	}

	details := map[string]any{"max_volume_m3": in.MaxVolumeM3, "max_weight_kg": in.MaxWeightKg}
	if err := writeAuditLog(ctx, tx, adminID, "platform_settings_updated", nil, details); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &in, nil
}
