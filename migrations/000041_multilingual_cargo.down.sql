ALTER TABLE consolidated_requests
    DROP COLUMN IF EXISTS destination_labels,
    DROP COLUMN IF EXISTS origin_labels;

ALTER TABLE participant_routes
    DROP COLUMN IF EXISTS destination_labels,
    DROP COLUMN IF EXISTS origin_labels;

ALTER TABLE cargo_requests
    DROP CONSTRAINT IF EXISTS cargo_requests_category_check,
    DROP COLUMN IF EXISTS destination_labels,
    DROP COLUMN IF EXISTS origin_labels,
    DROP COLUMN IF EXISTS category;
