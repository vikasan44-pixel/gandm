CREATE TABLE offers (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cargo_request_id       uuid NOT NULL REFERENCES cargo_requests (id) ON DELETE CASCADE,
    participant_id         uuid NOT NULL REFERENCES users (id),
    price                  numeric NOT NULL,
    currency               text NOT NULL DEFAULT 'KZT',
    conditions             text,
    warehouse_fill_percent numeric,
    status                 offer_status NOT NULL DEFAULT 'submitted',
    created_at             timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_offers_cargo_request_id ON offers (cargo_request_id);
CREATE INDEX idx_offers_participant_id ON offers (participant_id);
CREATE INDEX idx_offers_status ON offers (status);
