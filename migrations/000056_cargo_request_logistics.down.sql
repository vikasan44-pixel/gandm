DROP TABLE IF EXISTS cargo_request_items;

ALTER TABLE cargo_requests
    DROP COLUMN IF EXISTS packaging,
    DROP COLUMN IF EXISTS places_count,
    DROP COLUMN IF EXISTS stackable,
    DROP COLUMN IF EXISTS adr_required;
