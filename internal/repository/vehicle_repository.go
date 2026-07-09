package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gandm/internal/models"
)

type VehicleRepository struct {
	db Querier
}

func NewVehicleRepository(db Querier) *VehicleRepository {
	return &VehicleRepository{db: db}
}

const vehicleColumns = `id, user_id, axles, capacity_kg, length_m, width_m, height_m, body_type, current_location, created_at`

func scanVehicle(row pgx.Row) (*models.Vehicle, error) {
	var v models.Vehicle
	err := row.Scan(&v.ID, &v.UserID, &v.Axles, &v.CapacityKg, &v.LengthM, &v.WidthM, &v.HeightM, &v.BodyType, &v.CurrentLocation, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VehicleRepository) Create(ctx context.Context, v *models.Vehicle) error {
	const q = `
		INSERT INTO vehicles (id, user_id, axles, capacity_kg, length_m, width_m, height_m, body_type, current_location, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, q, v.ID, v.UserID, v.Axles, v.CapacityKg, v.LengthM, v.WidthM, v.HeightM, v.BodyType, v.CurrentLocation, v.CreatedAt)
	return err
}

func (r *VehicleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Vehicle, error) {
	q := `SELECT ` + vehicleColumns + ` FROM vehicles WHERE id = $1`
	return scanVehicle(r.db.QueryRow(ctx, q, id))
}

func (r *VehicleRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.Vehicle, error) {
	q := `SELECT ` + vehicleColumns + ` FROM vehicles WHERE user_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.Vehicle, 0)
	for rows.Next() {
		var v models.Vehicle
		if err := rows.Scan(&v.ID, &v.UserID, &v.Axles, &v.CapacityKg, &v.LengthM, &v.WidthM, &v.HeightM, &v.BodyType, &v.CurrentLocation, &v.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (r *VehicleRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM vehicles WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}

func (r *VehicleRepository) UpdateLocation(ctx context.Context, id uuid.UUID, location string) error {
	tag, err := r.db.Exec(ctx, `UPDATE vehicles SET current_location = $2 WHERE id = $1`, id, location)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *VehicleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
