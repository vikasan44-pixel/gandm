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

const vehicleColumns = `id, user_id, axles, capacity_kg, length_m, width_m, height_m, body_type, current_location,
	ready_origin_lat, ready_origin_lng, ready_origin_label, ready_origin_country,
	ready_destination_lat, ready_destination_lng, ready_destination_label, ready_destination_country,
	created_at`

// scanVehicleRow reads the flat columns and reassembles the optional ready
// direction into *GeoPoint (nil when the coordinate is NULL).
func scanVehicleRow(row pgx.Row, v *models.Vehicle) error {
	var oLat, oLng, dLat, dLng *float64
	var oLabel, oCountry, dLabel, dCountry string
	if err := row.Scan(
		&v.ID, &v.UserID, &v.Axles, &v.CapacityKg, &v.LengthM, &v.WidthM, &v.HeightM, &v.BodyType, &v.CurrentLocation,
		&oLat, &oLng, &oLabel, &oCountry,
		&dLat, &dLng, &dLabel, &dCountry,
		&v.CreatedAt,
	); err != nil {
		return err
	}
	if oLat != nil && oLng != nil {
		v.ReadyOrigin = &models.GeoPoint{Lat: *oLat, Lng: *oLng, Label: oLabel, Country: oCountry}
	}
	if dLat != nil && dLng != nil {
		v.ReadyDestination = &models.GeoPoint{Lat: *dLat, Lng: *dLng, Label: dLabel, Country: dCountry}
	}
	return nil
}

func scanVehicle(row pgx.Row) (*models.Vehicle, error) {
	var v models.Vehicle
	err := scanVehicleRow(row, &v)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// readyCoords unpacks an optional GeoPoint into nullable insert args.
func readyCoords(p *models.GeoPoint) (lat, lng *float64, label, country string) {
	if p == nil {
		return nil, nil, "", ""
	}
	lat, lng = &p.Lat, &p.Lng
	return lat, lng, p.Label, p.Country
}

func (r *VehicleRepository) Create(ctx context.Context, v *models.Vehicle) error {
	const q = `
		INSERT INTO vehicles (id, user_id, axles, capacity_kg, length_m, width_m, height_m, body_type, current_location,
			ready_origin_lat, ready_origin_lng, ready_origin_label, ready_origin_country,
			ready_destination_lat, ready_destination_lng, ready_destination_label, ready_destination_country, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`
	oLat, oLng, oLabel, oCountry := readyCoords(v.ReadyOrigin)
	dLat, dLng, dLabel, dCountry := readyCoords(v.ReadyDestination)
	_, err := r.db.Exec(ctx, q, v.ID, v.UserID, v.Axles, v.CapacityKg, v.LengthM, v.WidthM, v.HeightM, v.BodyType, v.CurrentLocation,
		oLat, oLng, oLabel, oCountry, dLat, dLng, dLabel, dCountry, v.CreatedAt)
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
		if err := scanVehicleRow(rows, &v); err != nil {
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
