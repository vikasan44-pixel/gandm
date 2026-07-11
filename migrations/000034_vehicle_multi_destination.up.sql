-- Автопарк, доработка по запросу владельца (2026-07-11):
--  • вместимость в кубометрах (capacity_m3) — отдельная графа к параметрам;
--  • местонахождение указывается КООРДИНАТАМИ (по карте), а не текстом —
--    заменяем свободный current_location на location_* (как груз/маршруты);
--  • назначений может быть НЕСКОЛЬКО (0..N) — выносим в отдельную таблицу
--    vehicle_destinations вместо единственного ready_destination.
-- Местонахождение играет роль «откуда» в публичном поиске по направлению,
-- назначения — «куда». Всё координатами → поиск по радиусу haversine.

ALTER TABLE vehicles
    ADD COLUMN capacity_m3      double precision NOT NULL DEFAULT 0,
    ADD COLUMN location_lat     double precision,
    ADD COLUMN location_lng     double precision,
    ADD COLUMN location_label   text NOT NULL DEFAULT '',
    ADD COLUMN location_country text NOT NULL DEFAULT '';

-- Старая модель направления (миграция 000033) и текстовое местонахождение
-- больше не нужны: origin теперь = location_*, назначения — отдельная таблица.
DROP INDEX IF EXISTS idx_vehicles_ready_origin_lat;
ALTER TABLE vehicles
    DROP COLUMN current_location,
    DROP COLUMN ready_origin_lat,
    DROP COLUMN ready_origin_lng,
    DROP COLUMN ready_origin_label,
    DROP COLUMN ready_origin_country,
    DROP COLUMN ready_destination_lat,
    DROP COLUMN ready_destination_lng,
    DROP COLUMN ready_destination_label,
    DROP COLUMN ready_destination_country;

-- Широтный префильтр публичного поиска «откуда» (как в миграции 000030).
CREATE INDEX idx_vehicles_location_lat ON vehicles (location_lat) WHERE location_lat IS NOT NULL;

-- Несколько назначений на машину. Координатами + подпись/страна.
CREATE TABLE vehicle_destinations (
    id         uuid PRIMARY KEY,
    vehicle_id uuid NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    lat        double precision NOT NULL,
    lng        double precision NOT NULL,
    label      text NOT NULL DEFAULT '',
    country    text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_vehicle_destinations_vehicle ON vehicle_destinations (vehicle_id);
CREATE INDEX idx_vehicle_destinations_lat ON vehicle_destinations (lat);
