ALTER TABLE vehicle_trips
    ADD COLUMN can_pickup_en_route boolean NOT NULL DEFAULT false,
    ADD COLUMN waypoints jsonb NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(waypoints) = 'array');
