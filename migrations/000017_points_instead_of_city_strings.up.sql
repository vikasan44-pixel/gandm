-- Switch cargo_requests and participant_routes from string city matching to
-- WGS-84 coordinate points. Coordinates are ALWAYS stored in WGS-84; any
-- GCJ-02 input (Amap) must be converted before it reaches the API.
--
-- Existing rows are test data with no coordinates to backfill from — they
-- are wiped (per the brief: recreate via seed). Offers and notifications
-- reference the wiped cargo requests, so they go too.

CREATE TYPE coord_source AS ENUM ('amap', 'osm');

TRUNCATE offers, notifications, cargo_requests, participant_routes;

ALTER TABLE cargo_requests RENAME COLUMN origin_city TO origin_label;
ALTER TABLE cargo_requests RENAME COLUMN destination_city TO destination_label;
ALTER TABLE cargo_requests
    ADD COLUMN origin_lat         numeric NOT NULL,
    ADD COLUMN origin_lng         numeric NOT NULL,
    ADD COLUMN origin_source      coord_source NOT NULL,
    ADD COLUMN destination_lat    numeric NOT NULL,
    ADD COLUMN destination_lng    numeric NOT NULL,
    ADD COLUMN destination_source coord_source NOT NULL;

-- The old (origin_city, destination_city) index served exact string
-- matching; with radius matching done on coordinates it has no query to
-- serve anymore.
DROP INDEX IF EXISTS idx_cargo_requests_route;

ALTER TABLE participant_routes RENAME COLUMN origin_city TO origin_label;
ALTER TABLE participant_routes RENAME COLUMN destination_city TO destination_label;
ALTER TABLE participant_routes
    ADD COLUMN origin_lat         numeric NOT NULL,
    ADD COLUMN origin_lng         numeric NOT NULL,
    ADD COLUMN origin_source      coord_source NOT NULL,
    ADD COLUMN destination_lat    numeric NOT NULL,
    ADD COLUMN destination_lng    numeric NOT NULL,
    ADD COLUMN destination_source coord_source NOT NULL;

ALTER TABLE participant_routes
    DROP CONSTRAINT IF EXISTS participant_routes_user_id_origin_city_destination_city_key;
DROP INDEX IF EXISTS idx_participant_routes_route;

-- "No duplicate routes" now means exact same coordinate pair. Two points a
-- few meters apart are technically different routes — dedupe-by-radius is a
-- product decision, not enforced here.
ALTER TABLE participant_routes
    ADD CONSTRAINT participant_routes_user_points_key
    UNIQUE (user_id, origin_lat, origin_lng, destination_lat, destination_lng);
