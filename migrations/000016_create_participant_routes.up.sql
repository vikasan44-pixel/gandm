CREATE TABLE participant_routes (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    origin_city      text NOT NULL,
    destination_city text NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, origin_city, destination_city)
);

CREATE INDEX idx_participant_routes_user_id ON participant_routes (user_id);
CREATE INDEX idx_participant_routes_route ON participant_routes (origin_city, destination_city);
