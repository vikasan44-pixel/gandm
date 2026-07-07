package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Well-known platform_settings keys.
const (
	SettingMaxVolumeM3 = "max_volume_m3"
	SettingMaxWeightKg = "max_weight_kg"
)

type SettingsRepository struct {
	db Querier
}

func NewSettingsRepository(db Querier) *SettingsRepository {
	return &SettingsRepository{db: db}
}

func (r *SettingsRepository) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRow(ctx, `SELECT value FROM platform_settings WHERE key = $1`, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return value, err
}

func (r *SettingsRepository) Set(ctx context.Context, key, value string) error {
	const q = `
		INSERT INTO platform_settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
	`
	_, err := r.db.Exec(ctx, q, key, value)
	return err
}
