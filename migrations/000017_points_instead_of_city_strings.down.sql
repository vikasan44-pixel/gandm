-- Structural rollback only: rows created while coordinates were live are
-- wiped, since the string-based schema can't represent them faithfully.

TRUNCATE offers, notifications, cargo_requests, participant_routes;

ALTER TABLE participant_routes
    DROP CONSTRAINT IF EXISTS participant_routes_user_points_key;
ALTER TABLE participant_routes
    DROP COLUMN IF EXISTS origin_lat,
    DROP COLUMN IF EXISTS origin_lng,
    DROP COLUMN IF EXISTS origin_source,
    DROP COLUMN IF EXISTS destination_lat,
    DROP COLUMN IF EXISTS destination_lng,
    DROP COLUMN IF EXISTS destination_source;
ALTER TABLE participant_routes RENAME COLUMN origin_label TO origin_city;
ALTER TABLE participant_routes RENAME COLUMN destination_label TO destination_city;
ALTER TABLE participant_routes
    ADD CONSTRAINT participant_routes_user_id_origin_city_destination_city_key
    UNIQUE (user_id, origin_city, destination_city);
CREATE INDEX idx_participant_routes_route ON participant_routes (origin_city, destination_city);

ALTER TABLE cargo_requests
    DROP COLUMN IF EXISTS origin_lat,
    DROP COLUMN IF EXISTS origin_lng,
    DROP COLUMN IF EXISTS origin_source,
    DROP COLUMN IF EXISTS destination_lat,
    DROP COLUMN IF EXISTS destination_lng,
    DROP COLUMN IF EXISTS destination_source;
ALTER TABLE cargo_requests RENAME COLUMN origin_label TO origin_city;
ALTER TABLE cargo_requests RENAME COLUMN destination_label TO destination_city;
CREATE INDEX idx_cargo_requests_route ON cargo_requests (origin_city, destination_city);

DROP TYPE IF EXISTS coord_source;
