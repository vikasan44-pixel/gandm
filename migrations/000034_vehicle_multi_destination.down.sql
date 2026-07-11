DROP TABLE IF EXISTS vehicle_destinations;

DROP INDEX IF EXISTS idx_vehicles_location_lat;

ALTER TABLE vehicles
    ADD COLUMN current_location          text NOT NULL DEFAULT '',
    ADD COLUMN ready_origin_lat          double precision,
    ADD COLUMN ready_origin_lng          double precision,
    ADD COLUMN ready_origin_label        text NOT NULL DEFAULT '',
    ADD COLUMN ready_origin_country      text NOT NULL DEFAULT '',
    ADD COLUMN ready_destination_lat     double precision,
    ADD COLUMN ready_destination_lng     double precision,
    ADD COLUMN ready_destination_label   text NOT NULL DEFAULT '',
    ADD COLUMN ready_destination_country text NOT NULL DEFAULT '';

CREATE INDEX idx_vehicles_ready_origin_lat ON vehicles (ready_origin_lat) WHERE ready_origin_lat IS NOT NULL;

ALTER TABLE vehicles
    DROP COLUMN capacity_m3,
    DROP COLUMN location_lat,
    DROP COLUMN location_lng,
    DROP COLUMN location_label,
    DROP COLUMN location_country;
