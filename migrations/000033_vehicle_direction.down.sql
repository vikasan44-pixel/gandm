DROP INDEX IF EXISTS idx_vehicles_ready_origin_lat;
ALTER TABLE vehicles
    DROP COLUMN ready_destination_country,
    DROP COLUMN ready_destination_label,
    DROP COLUMN ready_destination_lng,
    DROP COLUMN ready_destination_lat,
    DROP COLUMN ready_origin_country,
    DROP COLUMN ready_origin_label,
    DROP COLUMN ready_origin_lng,
    DROP COLUMN ready_origin_lat;
