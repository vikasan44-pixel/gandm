ALTER TABLE participant_routes
    DROP COLUMN IF EXISTS origin_country,
    DROP COLUMN IF EXISTS destination_country;

ALTER TABLE cargo_requests
    DROP COLUMN IF EXISTS origin_country,
    DROP COLUMN IF EXISTS destination_country;
