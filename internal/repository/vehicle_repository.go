package repository

import (
	"context"
	"errors"
	"time"

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

const vehicleColumns = `id, user_id, axles, capacity_kg, capacity_m3, length_m, width_m, height_m, body_type,
	location_lat, location_lng, location_label, location_country, created_at`

// scanVehicleRow reads the flat columns and reassembles the optional location
// into *GeoPoint (nil when the coordinate is NULL). Destinations are loaded
// separately (attachDestinations).
func scanVehicleRow(row pgx.Row, v *models.Vehicle) error {
	var lLat, lLng *float64
	var lLabel, lCountry string
	if err := row.Scan(
		&v.ID, &v.UserID, &v.Axles, &v.CapacityKg, &v.CapacityM3, &v.LengthM, &v.WidthM, &v.HeightM, &v.BodyType,
		&lLat, &lLng, &lLabel, &lCountry,
		&v.CreatedAt,
	); err != nil {
		return err
	}
	if lLat != nil && lLng != nil {
		v.Location = &models.GeoPoint{Lat: *lLat, Lng: *lLng, Label: lLabel, Country: lCountry}
	}
	v.Destinations = []models.VehicleDestination{}
	return nil
}

// locationCoords unpacks an optional GeoPoint into nullable insert args.
func locationCoords(p *models.GeoPoint) (lat, lng *float64, label, country string) {
	if p == nil {
		return nil, nil, "", ""
	}
	lat, lng = &p.Lat, &p.Lng
	return lat, lng, p.Label, p.Country
}

func (r *VehicleRepository) Create(ctx context.Context, v *models.Vehicle) error {
	const q = `
		INSERT INTO vehicles (id, user_id, axles, capacity_kg, capacity_m3, length_m, width_m, height_m, body_type,
			location_lat, location_lng, location_label, location_country, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	lLat, lLng, lLabel, lCountry := locationCoords(v.Location)
	_, err := r.db.Exec(ctx, q, v.ID, v.UserID, v.Axles, v.CapacityKg, v.CapacityM3, v.LengthM, v.WidthM, v.HeightM, v.BodyType,
		lLat, lLng, lLabel, lCountry, v.CreatedAt)
	return err
}

func (r *VehicleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Vehicle, error) {
	q := `SELECT ` + vehicleColumns + ` FROM vehicles WHERE id = $1`
	v, err := scanVehicle(r.db.QueryRow(ctx, q, id))
	if err != nil {
		return nil, err
	}
	if err := r.attachDestinations(ctx, []*models.Vehicle{v}); err != nil {
		return nil, err
	}
	return v, nil
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ptrs := make([]*models.Vehicle, len(items))
	for i := range items {
		ptrs[i] = &items[i]
	}
	if err := r.attachDestinations(ctx, ptrs); err != nil {
		return nil, err
	}
	return items, nil
}

// attachDestinations loads all destinations for the given vehicles in one
// query and fills each vehicle's Destinations slice (empty if none).
func (r *VehicleRepository) attachDestinations(ctx context.Context, vehicles []*models.Vehicle) error {
	if len(vehicles) == 0 {
		return nil
	}
	byID := make(map[uuid.UUID]*models.Vehicle, len(vehicles))
	ids := make([]uuid.UUID, 0, len(vehicles))
	for _, v := range vehicles {
		byID[v.ID] = v
		ids = append(ids, v.ID)
	}
	rows, err := r.db.Query(ctx,
		`SELECT vehicle_id, id, lat, lng, label, country
		 FROM vehicle_destinations WHERE vehicle_id = ANY($1) ORDER BY created_at ASC`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var vid uuid.UUID
		var d models.VehicleDestination
		if err := rows.Scan(&vid, &d.ID, &d.Point.Lat, &d.Point.Lng, &d.Point.Label, &d.Point.Country); err != nil {
			return err
		}
		if v := byID[vid]; v != nil {
			v.Destinations = append(v.Destinations, d)
		}
	}
	return rows.Err()
}

func (r *VehicleRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM vehicles WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}

// UpdateLocation sets the vehicle's map location (or clears it when p is nil).
func (r *VehicleRepository) UpdateLocation(ctx context.Context, id uuid.UUID, p *models.GeoPoint) error {
	lat, lng, label, country := locationCoords(p)
	tag, err := r.db.Exec(ctx,
		`UPDATE vehicles SET location_lat = $2, location_lng = $3, location_label = $4, location_country = $5 WHERE id = $1`,
		id, lat, lng, label, country)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddDestination appends one destination to a vehicle and returns it (with id).
func (r *VehicleRepository) AddDestination(ctx context.Context, vehicleID uuid.UUID, p models.GeoPoint) (models.VehicleDestination, error) {
	id := uuid.New()
	_, err := r.db.Exec(ctx,
		`INSERT INTO vehicle_destinations (id, vehicle_id, lat, lng, label, country, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, vehicleID, p.Lat, p.Lng, p.Label, p.Country, time.Now())
	if err != nil {
		return models.VehicleDestination{}, err
	}
	return models.VehicleDestination{ID: id, Point: p}, nil
}

// DeleteDestination removes one destination that belongs to the given vehicle.
func (r *VehicleRepository) DeleteDestination(ctx context.Context, vehicleID, destID uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM vehicle_destinations WHERE id = $1 AND vehicle_id = $2`, destID, vehicleID)
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
