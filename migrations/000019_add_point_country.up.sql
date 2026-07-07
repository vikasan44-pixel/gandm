-- Per-country matching radii (CN 100 km / KZ+other 40 km) need each point's
-- country. Country is filled by the frontend from the geocoder (Nominatim
-- address.country_code for OSM; Amap points are always "cn") — NOT derived
-- from a coordinate bounding box. Lowercase ISO-3166 alpha-2 ("cn", "kz",
-- ...); empty string = unknown, which matching treats as the default
-- (non-China) radius.
--
-- Existing rows are test data with no country to backfill from — wiped and
-- recreated via seed, same as migration 000017 did.

TRUNCATE offers, notifications, cargo_requests, participant_routes;

ALTER TABLE cargo_requests
    ADD COLUMN origin_country      text NOT NULL,
    ADD COLUMN destination_country text NOT NULL;

ALTER TABLE participant_routes
    ADD COLUMN origin_country      text NOT NULL,
    ADD COLUMN destination_country text NOT NULL;
