-- Consolidation (Stage 5): capacity limits editable at runtime, pair
-- suggestions, consolidated requests, and offers targeting either a single
-- cargo request or a consolidated one (exactly one of the two).

CREATE TABLE platform_settings (
    key   text PRIMARY KEY,
    value text NOT NULL
);

INSERT INTO platform_settings (key, value) VALUES
    ('max_volume_m3', '90'),
    ('max_weight_kg', '20000');

CREATE TYPE consolidation_status AS ENUM (
    'suggested',
    'a_agreed',
    'b_agreed',
    'both_agreed',
    'declined'
);

CREATE TABLE consolidation_suggestions (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cargo_request_a uuid NOT NULL REFERENCES cargo_requests (id) ON DELETE CASCADE,
    cargo_request_b uuid NOT NULL REFERENCES cargo_requests (id) ON DELETE CASCADE,
    direction_label text NOT NULL,
    status          consolidation_status NOT NULL DEFAULT 'suggested',
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_consolidation_suggestions_a ON consolidation_suggestions (cargo_request_a);
CREATE INDEX idx_consolidation_suggestions_b ON consolidation_suggestions (cargo_request_b);

-- label/country columns are an addition to the brief's schema: labels are
-- needed to show the route to humans, countries to apply the per-country
-- matching radius. Reuses the cargo_request_status enum (same lifecycle).
CREATE TABLE consolidated_requests (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    origin_lat          numeric NOT NULL,
    origin_lng          numeric NOT NULL,
    origin_label        text NOT NULL,
    origin_country      text NOT NULL,
    destination_lat     numeric NOT NULL,
    destination_lng     numeric NOT NULL,
    destination_label   text NOT NULL,
    destination_country text NOT NULL,
    total_volume_m3     numeric NOT NULL,
    total_weight_kg     numeric NOT NULL,
    member_request_ids  jsonb NOT NULL,
    status              cargo_request_status NOT NULL DEFAULT 'open',
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_consolidated_requests_status ON consolidated_requests (status);

-- An offer now targets either a single cargo request or a consolidated one.
ALTER TABLE offers ALTER COLUMN cargo_request_id DROP NOT NULL;
ALTER TABLE offers ADD COLUMN consolidated_request_id uuid REFERENCES consolidated_requests (id) ON DELETE CASCADE;
ALTER TABLE offers ADD CONSTRAINT offers_exactly_one_target
    CHECK (num_nonnulls(cargo_request_id, consolidated_request_id) = 1);
CREATE INDEX idx_offers_consolidated_request_id ON offers (consolidated_request_id);
