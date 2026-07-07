CREATE TABLE cargo_requests (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id        uuid NOT NULL REFERENCES users (id),
    origin_city      text NOT NULL,
    destination_city text NOT NULL,
    volume_m3        numeric NOT NULL,
    weight_kg        numeric NOT NULL,
    description      text,
    status           cargo_request_status NOT NULL DEFAULT 'open',
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_cargo_requests_client_id ON cargo_requests (client_id);
CREATE INDEX idx_cargo_requests_status ON cargo_requests (status);
CREATE INDEX idx_cargo_requests_route ON cargo_requests (origin_city, destination_city);
