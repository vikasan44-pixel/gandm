package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"gandm/internal/geo"
	"gandm/internal/models"
)

// Публичный (гостевой) поиск грузов и транспорта. Ключевой принцип платформы:
// НИКАКОГО текстового сопоставления городов — разные люди пишут «Алматы» /
// «Almaty» / «阿拉木图» по-разному. Гость выбирает точку через геокодер, и поиск
// идёт по координатам + радиусу haversine, ровно как консолидация и матчинг.
//
// appendRadiusFilter дописывает в запрос условие «колонка-точка в радиусе от p».
// Сначала широтная полоса (sargable, ловит btree-индекс из миграции 000030),
// затем точный haversine. Радиус берётся GREATEST по стране обеих точек: cn —
// шире, kz/пусто/прочее — уже. Все аргументы (включая cnKm/kzKm) добавляются
// самим хелпером — так в запросе не остаётся неиспользуемых плейсхолдеров,
// когда фильтр по точке не применяется вовсе.
func appendRadiusFilter(q *strings.Builder, args *[]any, p *models.GeoPoint, latCol, lngCol, countryCol string, cnKm, kzKm float64) {
	*args = append(*args, cnKm, kzKm, p.Lat, p.Lng, p.Country)
	cnIdx := len(*args) - 4
	kzIdx := len(*args) - 3
	latIdx := len(*args) - 2
	lngIdx := len(*args) - 1
	ctryIdx := len(*args)
	fmt.Fprintf(q, `
		AND %[1]s BETWEEN $%[2]d::float8 - GREATEST($%[7]d::float8, $%[8]d::float8) / 110.0
		              AND $%[2]d::float8 + GREATEST($%[7]d::float8, $%[8]d::float8) / 110.0
		AND haversine_km($%[2]d::float8, $%[3]d::float8, %[1]s, %[4]s)
		    <= GREATEST(
		         CASE WHEN %[5]s = 'cn' THEN $%[7]d::float8 ELSE $%[8]d::float8 END,
		         CASE WHEN $%[6]d = 'cn' THEN $%[7]d::float8 ELSE $%[8]d::float8 END)`,
		latCol, latIdx, lngIdx, lngCol, countryCol, ctryIdx, cnIdx, kzIdx)
}

// SearchOpenPublicCargo — открытые заявки на груз для гостевого поиска.
// from/to опциональны: указана только точка «откуда» — фильтр по origin,
// только «куда» — по destination, обе — по обеим. Возвращает не более 200
// свежих записей. Анонимизацию (срез client_id/description) делает сервис.
func (r *CargoRequestRepository) SearchOpenPublicCargo(ctx context.Context, from, to *models.GeoPoint, excludeClientID *uuid.UUID, cnKm, kzKm float64) ([]models.CargoRequest, error) {
	items, _, err := r.SearchOpenPublicCargoPage(ctx, from, to, excludeClientID, cnKm, kzKm, 200, 0)
	return items, err
}

func publicCargoSearchWhere(from, to *models.GeoPoint, excludeClientID *uuid.UUID, cnKm, kzKm float64) (string, []any) {
	var q strings.Builder
	q.WriteString(` WHERE cr.status = 'open'`)
	args := []any{}
	if excludeClientID != nil {
		args = append(args, *excludeClientID)
		fmt.Fprintf(&q, ` AND cr.client_id <> $%d`, len(args))
	}
	if from != nil {
		appendRadiusFilter(&q, &args, from, "cr.origin_lat", "cr.origin_lng", "cr.origin_country", cnKm, kzKm)
	}
	if to != nil {
		appendRadiusFilter(&q, &args, to, "cr.destination_lat", "cr.destination_lng", "cr.destination_country", cnKm, kzKm)
	}
	return q.String(), args
}

func (r *CargoRequestRepository) SearchOpenPublicCargoPage(ctx context.Context, from, to *models.GeoPoint, excludeClientID *uuid.UUID, cnKm, kzKm float64, limit, offset int) ([]models.CargoRequest, int, error) {
	where, args := publicCargoSearchWhere(from, to, excludeClientID, cnKm, kzKm)
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM cargo_requests cr`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	query := `SELECT ` + cargoRequestColumns + ` FROM cargo_requests cr` + where +
		fmt.Sprintf(` ORDER BY cr.created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))
	items, err := queryCargoRequests(ctx, r.db, query, args...)
	return items, total, err
}

// VehicleSearchFilter — параметры гостевого поиска транспорта: характеристики
// (пустое/ноль = «любой») и опциональное направление координатами.
type VehicleSearchFilter struct {
	BodyType      string
	MinCapacityKg float64
	MinCapacityM3 float64
	MinLengthM    float64
	MinWidthM     float64
	MinHeightM    float64
	MinAxles      int
	From          *models.GeoPoint // местонахождение (откуда)
	To            *models.GeoPoint // одно из назначений (куда)
}

// SearchPublicVehicles — транспорт для гостевого поиска: по характеристикам и,
// опционально, по направлению координатами+радиусом. «Откуда» сверяется с
// местонахождением машины (location), «куда» — с ЛЮБЫМ из её назначений
// (EXISTS по vehicle_destinations). Машина без местонахождения не попадёт в
// выдачу с фильтром «откуда», без назначений — с фильтром «куда»: публично
// по направлению находятся только те, кто его указал.
func (r *VehicleRepository) SearchPublicVehicles(ctx context.Context, f VehicleSearchFilter, excludeUserID *uuid.UUID, cnKm, kzKm float64) ([]models.Vehicle, error) {
	var q strings.Builder
	q.WriteString(`SELECT ` + vehicleColumns + ` FROM vehicles WHERE EXISTS (
		SELECT 1 FROM users owner WHERE owner.id = vehicles.user_id AND owner.status = 'active'
	)`)
	args := []any{}

	addScalar := func(cond string, val any) {
		args = append(args, val)
		fmt.Fprintf(&q, cond, len(args))
	}
	if excludeUserID != nil {
		addScalar(` AND user_id <> $%d`, *excludeUserID)
	}
	if f.BodyType != "" {
		addScalar(` AND body_type = $%d`, f.BodyType)
	}
	if f.MinCapacityKg > 0 {
		addScalar(` AND capacity_kg >= $%d`, f.MinCapacityKg)
	}
	if f.MinCapacityM3 > 0 {
		addScalar(` AND capacity_m3 >= $%d`, f.MinCapacityM3)
	}
	if f.MinLengthM > 0 {
		addScalar(` AND length_m >= $%d`, f.MinLengthM)
	}
	if f.MinWidthM > 0 {
		addScalar(` AND width_m >= $%d`, f.MinWidthM)
	}
	if f.MinHeightM > 0 {
		addScalar(` AND height_m >= $%d`, f.MinHeightM)
	}
	if f.MinAxles > 0 {
		addScalar(` AND axles >= $%d`, f.MinAxles)
	}
	// Direction matching is finalized after active trips are attached. This
	// allows ordered waypoint matching (pickup point must occur before the
	// delivery point) without duplicating JSON route logic in SQL.
	q.WriteString(` ORDER BY created_at DESC LIMIT 1000`)

	rows, err := r.db.Query(ctx, q.String(), args...)
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
	matched := make([]models.Vehicle, 0, len(items))
	for _, vehicle := range items {
		searchableTrips := searchableVehicleTrips(vehicle.Trips)
		if len(searchableTrips) > 0 {
			for _, trip := range searchableTrips {
				if !tripMatchesDirection(trip, f.From, f.To) {
					continue
				}
				if f.MinCapacityKg > 0 && vehicle.CapacityKg-trip.LoadedWeightKg < f.MinCapacityKg {
					continue
				}
				if f.MinCapacityM3 > 0 && vehicle.CapacityM3-trip.LoadedVolumeM3 < f.MinCapacityM3 {
					continue
				}
				candidate := vehicle
				candidate.Trips = []models.VehicleTrip{trip}
				matched = append(matched, candidate)
				if len(matched) == 200 {
					return matched, nil
				}
			}
			continue
		}

		if !vehicleMatchesLegacyDirection(vehicle, f.From, f.To, cnKm, kzKm) {
			continue
		}
		matched = append(matched, vehicle)
		if len(matched) == 200 {
			break
		}
	}
	return matched, nil
}

func committedVehicleTrip(trips []models.VehicleTrip) *models.VehicleTrip {
	for index := range trips {
		if trips[index].HasActiveCargo() {
			return &trips[index]
		}
	}
	return nil
}

func searchableVehicleTrips(trips []models.VehicleTrip) []models.VehicleTrip {
	if committed := committedVehicleTrip(trips); committed != nil {
		return []models.VehicleTrip{*committed}
	}
	plans := make([]models.VehicleTrip, 0, len(trips))
	for _, trip := range trips {
		if trip.Status == models.VehicleTripPlanned {
			plans = append(plans, trip)
		}
	}
	return plans
}

func tripMatchesDirection(trip models.VehicleTrip, from, to *models.GeoPoint) bool {
	if from == nil && to == nil {
		return true
	}
	radius := 50.0
	points := []models.GeoPoint{trip.Origin}
	if trip.CanPickupEnRoute {
		points = append(points, trip.Waypoints...)
	}
	points = append(points, trip.Destination)
	if from != nil && to != nil {
		for fromIndex, point := range points {
			if !pointNear(point, from, radius) {
				continue
			}
			for toIndex := fromIndex + 1; toIndex < len(points); toIndex++ {
				if pointNear(points[toIndex], to, radius) {
					return true
				}
			}
		}
		return false
	}
	if from != nil {
		for index, point := range points {
			if index < len(points)-1 && pointNear(point, from, radius) {
				return true
			}
		}
		return false
	}
	for index := 1; index < len(points); index++ {
		if pointNear(points[index], to, radius) {
			return true
		}
	}
	return false
}

func pointRadiusKm(left, right models.GeoPoint, cnKm, kzKm float64) float64 {
	if strings.EqualFold(left.Country, "cn") || strings.EqualFold(right.Country, "cn") {
		return cnKm
	}
	return kzKm
}

func pointNear(left models.GeoPoint, right *models.GeoPoint, radiusKm float64) bool {
	return right != nil && geo.HaversineKm(left.Lat, left.Lng, right.Lat, right.Lng) <= radiusKm
}

func vehicleMatchesDirection(vehicle models.Vehicle, from, to *models.GeoPoint, cnKm, kzKm float64) bool {
	for _, trip := range searchableVehicleTrips(vehicle.Trips) {
		if tripMatchesDirection(trip, from, to) {
			return true
		}
	}
	if len(searchableVehicleTrips(vehicle.Trips)) > 0 {
		return false
	}
	return vehicleMatchesLegacyDirection(vehicle, from, to, cnKm, kzKm)
}

func vehicleMatchesLegacyDirection(vehicle models.Vehicle, from, to *models.GeoPoint, cnKm, kzKm float64) bool {
	// Vehicles without an active plan keep the previous location/destination
	// matching behavior.
	if from != nil {
		if vehicle.Location == nil || !pointNear(*vehicle.Location, from, pointRadiusKm(*vehicle.Location, *from, cnKm, kzKm)) {
			return false
		}
	}
	if to != nil {
		matched := false
		for _, destination := range vehicle.Destinations {
			if pointNear(destination.Point, to, pointRadiusKm(destination.Point, *to, cnKm, kzKm)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
