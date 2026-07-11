package repository

import (
	"context"
	"fmt"
	"strings"

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
func (r *CargoRequestRepository) SearchOpenPublicCargo(ctx context.Context, from, to *models.GeoPoint, cnKm, kzKm float64) ([]models.CargoRequest, error) {
	var q strings.Builder
	q.WriteString(`SELECT ` + cargoRequestColumns + ` FROM cargo_requests cr WHERE cr.status = 'open'`)
	args := []any{}
	if from != nil {
		appendRadiusFilter(&q, &args, from, "cr.origin_lat", "cr.origin_lng", "cr.origin_country", cnKm, kzKm)
	}
	if to != nil {
		appendRadiusFilter(&q, &args, to, "cr.destination_lat", "cr.destination_lng", "cr.destination_country", cnKm, kzKm)
	}
	q.WriteString(` ORDER BY cr.created_at DESC LIMIT 200`)
	return queryCargoRequests(ctx, r.db, q.String(), args...)
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
func (r *VehicleRepository) SearchPublicVehicles(ctx context.Context, f VehicleSearchFilter, cnKm, kzKm float64) ([]models.Vehicle, error) {
	var q strings.Builder
	q.WriteString(`SELECT ` + vehicleColumns + ` FROM vehicles WHERE TRUE`)
	args := []any{}

	addScalar := func(cond string, val any) {
		args = append(args, val)
		fmt.Fprintf(&q, cond, len(args))
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
	if f.From != nil {
		appendRadiusFilter(&q, &args, f.From, "location_lat", "location_lng", "location_country", cnKm, kzKm)
	}
	if f.To != nil {
		q.WriteString(` AND EXISTS (SELECT 1 FROM vehicle_destinations d WHERE d.vehicle_id = vehicles.id`)
		appendRadiusFilter(&q, &args, f.To, "d.lat", "d.lng", "d.country", cnKm, kzKm)
		q.WriteString(`)`)
	}
	q.WriteString(` ORDER BY created_at DESC LIMIT 200`)

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
	return items, nil
}
