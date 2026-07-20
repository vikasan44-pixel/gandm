package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"gandm/internal/models"
)

// marshalLabels/scanLabels move the ru/en/zh label map in and out of a jsonb
// column. Empty/absent → NULL (nil) so points without translations stay lean.
func marshalLabels(m map[string]string) []byte {
	if len(m) == 0 {
		return nil
	}
	b, _ := json.Marshal(m)
	return b
}

func scanLabels(raw []byte) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]string
	_ = json.Unmarshal(raw, &m)
	return m
}

type VehicleRepository struct {
	db Querier
}

var (
	ErrVehicleIdentityTaken    = errors.New("vehicle plate or VIN already registered")
	ErrVehicleTripDateConflict = errors.New("vehicle already has a trip on this date")
	ErrVehicleActiveTrip       = errors.New("vehicle already has an active trip")
)

func NewVehicleRepository(db Querier) *VehicleRepository {
	return &VehicleRepository{db: db}
}

const vehicleColumns = `id, user_id, name, axles, capacity_kg, capacity_m3, length_m, width_m, height_m, body_type,
	registration_country, plate_number, vin, verification_status, privacy_consent_at, privacy_consent_version,
	verified_at, verification_reject_reason,
	location_lat, location_lng, location_label, location_country, location_labels, created_at`

// scanVehicleRow reads the flat columns and reassembles the optional location
// into *GeoPoint (nil when the coordinate is NULL). Destinations are loaded
// separately (attachDestinations).
func scanVehicleRow(row pgx.Row, v *models.Vehicle) error {
	var lLat, lLng *float64
	var lLabel, lCountry string
	var lLabels []byte
	if err := row.Scan(
		&v.ID, &v.UserID, &v.Name, &v.Axles, &v.CapacityKg, &v.CapacityM3, &v.LengthM, &v.WidthM, &v.HeightM, &v.BodyType,
		&v.RegistrationCountry, &v.PlateNumber, &v.VIN, &v.VerificationStatus, &v.PrivacyConsentAt, &v.PrivacyConsentVersion,
		&v.VerifiedAt, &v.VerificationRejectReason,
		&lLat, &lLng, &lLabel, &lCountry, &lLabels,
		&v.CreatedAt,
	); err != nil {
		return err
	}
	if lLat != nil && lLng != nil {
		v.Location = &models.GeoPoint{Lat: *lLat, Lng: *lLng, Label: lLabel, Country: lCountry, Labels: scanLabels(lLabels)}
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
		INSERT INTO vehicles (id, user_id, name, axles, capacity_kg, capacity_m3, length_m, width_m, height_m, body_type,
			registration_country, plate_number, vin, verification_status, privacy_consent_at, privacy_consent_version,
			location_lat, location_lng, location_label, location_country, location_labels, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
	`
	lLat, lLng, lLabel, lCountry := locationCoords(v.Location)
	var lLabels []byte
	if v.Location != nil {
		lLabels = marshalLabels(v.Location.Labels)
	}
	_, err := r.db.Exec(ctx, q, v.ID, v.UserID, v.Name, v.Axles, v.CapacityKg, v.CapacityM3, v.LengthM, v.WidthM, v.HeightM, v.BodyType,
		v.RegistrationCountry, v.PlateNumber, v.VIN, v.VerificationStatus, v.PrivacyConsentAt, v.PrivacyConsentVersion,
		lLat, lLng, lLabel, lCountry, lLabels, v.CreatedAt)
	return vehicleIdentityError(err)
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
	if err := r.attachTrips(ctx, []*models.Vehicle{v}); err != nil {
		return nil, err
	}
	if err := r.attachVerificationSummary(ctx, []*models.Vehicle{v}); err != nil {
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
	if err := r.attachTrips(ctx, ptrs); err != nil {
		return nil, err
	}
	if err := r.attachVerificationSummary(ctx, ptrs); err != nil {
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
		`SELECT vehicle_id, id, lat, lng, label, country, labels
		 FROM vehicle_destinations WHERE vehicle_id = ANY($1) ORDER BY created_at ASC`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var vid uuid.UUID
		var d models.VehicleDestination
		var labels []byte
		if err := rows.Scan(&vid, &d.ID, &d.Point.Lat, &d.Point.Lng, &d.Point.Label, &d.Point.Country, &labels); err != nil {
			return err
		}
		d.Point.Labels = scanLabels(labels)
		if v := byID[vid]; v != nil {
			v.Destinations = append(v.Destinations, d)
		}
	}
	return rows.Err()
}

func scanVehicleTrip(row pgx.Row) (*models.VehicleTrip, error) {
	var trip models.VehicleTrip
	var origin, destination, waypoints []byte
	err := row.Scan(
		&trip.ID, &trip.VehicleID, &origin, &destination, &waypoints, &trip.CanPickupEnRoute, &trip.DepartureDate,
		&trip.LoadedWeightKg, &trip.LoadedVolumeM3, &trip.Status,
		&trip.CreatedAt, &trip.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(origin, &trip.Origin); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(destination, &trip.Destination); err != nil {
		return nil, err
	}
	trip.Waypoints = []models.GeoPoint{}
	if err := json.Unmarshal(waypoints, &trip.Waypoints); err != nil {
		return nil, err
	}
	return &trip, nil
}

func (r *VehicleRepository) attachTrips(ctx context.Context, vehicles []*models.Vehicle) error {
	if len(vehicles) == 0 {
		return nil
	}
	byID := make(map[uuid.UUID]*models.Vehicle, len(vehicles))
	ids := make([]uuid.UUID, 0, len(vehicles))
	for _, vehicle := range vehicles {
		vehicle.Trips = []models.VehicleTrip{}
		byID[vehicle.ID] = vehicle
		ids = append(ids, vehicle.ID)
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, vehicle_id, origin, destination, waypoints, can_pickup_en_route, departure_date,
		       loaded_weight_kg, loaded_volume_m3, status, created_at, updated_at
		FROM vehicle_trips
		WHERE vehicle_id = ANY($1)
		ORDER BY departure_date DESC, created_at DESC`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		trip, err := scanVehicleTrip(rows)
		if err != nil {
			return err
		}
		if vehicle := byID[trip.VehicleID]; vehicle != nil {
			vehicle.Trips = append(vehicle.Trips, *trip)
		}
	}
	return rows.Err()
}

func (r *VehicleRepository) attachVerificationSummary(ctx context.Context, vehicles []*models.Vehicle) error {
	if len(vehicles) == 0 {
		return nil
	}
	byID := make(map[uuid.UUID]*models.Vehicle, len(vehicles))
	ids := make([]uuid.UUID, 0, len(vehicles))
	for _, vehicle := range vehicles {
		vehicle.UploadedDocumentTypes = []models.VehicleDocumentType{}
		byID[vehicle.ID] = vehicle
		ids = append(ids, vehicle.ID)
	}
	rows, err := r.db.Query(ctx, `
		SELECT document.vehicle_id,
		       COALESCE(array_agg(document.type ORDER BY document.type) FILTER (WHERE document.type IS NOT NULL), ARRAY[]::text[]),
		       EXISTS (SELECT 1 FROM vehicle_trips trip WHERE trip.vehicle_id = document.vehicle_id AND trip.status = 'completed')
		FROM vehicle_documents document
		WHERE document.vehicle_id = ANY($1)
		GROUP BY document.vehicle_id`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var vehicleID uuid.UUID
		var types []string
		var hasHistory bool
		if err := rows.Scan(&vehicleID, &types, &hasHistory); err != nil {
			return err
		}
		if vehicle := byID[vehicleID]; vehicle != nil {
			vehicle.HasCompletedTrips = hasHistory
			for _, docType := range types {
				vehicle.UploadedDocumentTypes = append(vehicle.UploadedDocumentTypes, models.VehicleDocumentType(docType))
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Vehicles without documents are absent from the GROUP BY, but may still
	// have completed trips. Fill that history flag in one bounded query.
	historyRows, err := r.db.Query(ctx, `SELECT DISTINCT vehicle_id FROM vehicle_trips WHERE vehicle_id = ANY($1) AND status = 'completed'`, ids)
	if err != nil {
		return err
	}
	defer historyRows.Close()
	for historyRows.Next() {
		var vehicleID uuid.UUID
		if err := historyRows.Scan(&vehicleID); err != nil {
			return err
		}
		if vehicle := byID[vehicleID]; vehicle != nil {
			vehicle.HasCompletedTrips = true
		}
	}
	if err := historyRows.Err(); err != nil {
		return err
	}
	for _, vehicle := range vehicles {
		vehicle.DocumentsVerified = vehicle.VerificationStatus == models.VehicleVerificationVerified
		switch {
		case vehicle.DocumentsVerified && vehicle.HasCompletedTrips:
			vehicle.TrustPercent = 100
		case vehicle.DocumentsVerified:
			vehicle.TrustPercent = 90
		case vehicle.HasCompletedTrips:
			vehicle.TrustPercent = 70
		default:
			vehicle.TrustPercent = 50
		}
		vehicle.MaskedPlate = maskVehiclePlate(vehicle.PlateNumber)
	}
	return nil
}

func maskVehiclePlate(plate string) string {
	plate = strings.TrimSpace(plate)
	runes := []rune(plate)
	if len(runes) == 0 {
		return ""
	}
	visible := 3
	if len(runes) < visible {
		visible = len(runes)
	}
	return "*** " + string(runes[len(runes)-visible:])
}

func vehicleIdentityError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		(pgErr.ConstraintName == "idx_vehicles_country_plate_unique" || pgErr.ConstraintName == "idx_vehicles_vin_unique") {
		return ErrVehicleIdentityTaken
	}
	return err
}

func vehicleTripError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "vehicle_trip_date_conflict" {
		return ErrVehicleTripDateConflict
	}
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "vehicle_active_trip_conflict" {
		return ErrVehicleActiveTrip
	}
	return err
}

func (r *VehicleRepository) UpdateRegistration(ctx context.Context, vehicleID uuid.UUID, country, plate, vin, consentVersion string, consentAt time.Time) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE vehicles
		SET registration_country=$2, plate_number=$3, vin=$4,
		    privacy_consent_at=$5, privacy_consent_version=$6,
		    verification_status='not_submitted', verified_at=NULL, verified_by=NULL,
		    verification_reject_reason=NULL
		WHERE id=$1`, vehicleID, country, plate, vin, consentAt, consentVersion)
	if err != nil {
		return vehicleIdentityError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *VehicleRepository) RefreshVerificationStatus(ctx context.Context, vehicleID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE vehicles vehicle
		SET verification_status = CASE
		    WHEN vehicle.registration_country <> '' AND vehicle.plate_number <> '' AND vehicle.vin <> ''
		     AND vehicle.privacy_consent_at IS NOT NULL
		     AND (SELECT count(DISTINCT document.type) FROM vehicle_documents document WHERE document.vehicle_id=vehicle.id) = 7
		    THEN 'pending' ELSE 'not_submitted' END,
		    verified_at=NULL, verified_by=NULL, verification_reject_reason=NULL
		WHERE vehicle.id=$1`, vehicleID)
	return err
}

func (r *VehicleRepository) SetVerificationResult(ctx context.Context, vehicleID, adminID uuid.UUID, status models.VehicleVerificationStatus, reason *string, now time.Time) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE vehicles
		SET verification_status=$3, verified_at=CASE WHEN $3='verified' THEN $5 ELSE NULL END,
		    verified_by=$2, verification_reject_reason=$4
		WHERE id=$1 AND verification_status='pending'`, vehicleID, adminID, status, reason, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *VehicleRepository) CreateTrip(ctx context.Context, trip *models.VehicleTrip) error {
	origin, err := json.Marshal(trip.Origin)
	if err != nil {
		return err
	}
	destination, err := json.Marshal(trip.Destination)
	if err != nil {
		return err
	}
	waypoints, err := json.Marshal(trip.Waypoints)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO vehicle_trips (
			id, vehicle_id, origin, destination, waypoints, can_pickup_en_route, departure_date,
			loaded_weight_kg, loaded_volume_m3, status, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		trip.ID, trip.VehicleID, origin, destination, waypoints, trip.CanPickupEnRoute, trip.DepartureDate,
		trip.LoadedWeightKg, trip.LoadedVolumeM3, trip.Status, trip.CreatedAt, trip.UpdatedAt)
	return vehicleTripError(err)
}

func (r *VehicleRepository) UpdateTrip(ctx context.Context, trip *models.VehicleTrip) error {
	origin, err := json.Marshal(trip.Origin)
	if err != nil {
		return err
	}
	destination, err := json.Marshal(trip.Destination)
	if err != nil {
		return err
	}
	waypoints, err := json.Marshal(trip.Waypoints)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE vehicle_trips
		SET origin=$3, destination=$4, waypoints=$5, can_pickup_en_route=$6, departure_date=$7,
		    loaded_weight_kg=$8, loaded_volume_m3=$9, status=$10, updated_at=$11
		WHERE id=$1 AND vehicle_id=$2`,
		trip.ID, trip.VehicleID, origin, destination, waypoints, trip.CanPickupEnRoute, trip.DepartureDate,
		trip.LoadedWeightKg, trip.LoadedVolumeM3, trip.Status, trip.UpdatedAt)
	if err != nil {
		return vehicleTripError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *VehicleRepository) DeleteTrip(ctx context.Context, vehicleID, tripID uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM vehicle_trips WHERE id=$1 AND vehicle_id=$2`, tripID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *VehicleRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM vehicles WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}

// UpdateLocation sets the vehicle's map location (or clears it when p is nil).
func (r *VehicleRepository) UpdateLocation(ctx context.Context, id uuid.UUID, p *models.GeoPoint) error {
	lat, lng, label, country := locationCoords(p)
	var labels []byte
	if p != nil {
		labels = marshalLabels(p.Labels)
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE vehicles SET location_lat = $2, location_lng = $3, location_label = $4, location_country = $5, location_labels = $6 WHERE id = $1`,
		id, lat, lng, label, country, labels)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *VehicleRepository) UpdateName(ctx context.Context, id uuid.UUID, name string) error {
	tag, err := r.db.Exec(ctx, `UPDATE vehicles SET name = $2 WHERE id = $1`, id, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *VehicleRepository) UpdateDetails(ctx context.Context, vehicle *models.Vehicle) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE vehicles
		SET name=$2, axles=$3, capacity_kg=$4, capacity_m3=$5,
		    length_m=$6, width_m=$7, height_m=$8, body_type=$9
		WHERE id=$1`,
		vehicle.ID, vehicle.Name, vehicle.Axles, vehicle.CapacityKg, vehicle.CapacityM3,
		vehicle.LengthM, vehicle.WidthM, vehicle.HeightM, vehicle.BodyType)
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
		`INSERT INTO vehicle_destinations (id, vehicle_id, lat, lng, label, country, labels, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, vehicleID, p.Lat, p.Lng, p.Label, p.Country, marshalLabels(p.Labels), time.Now())
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
