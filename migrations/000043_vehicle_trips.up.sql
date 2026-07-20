-- Загрузка относится не к машине вообще, а к конкретному рейсу машины.
-- Одна машина может выполнять несколько рейсов по одному направлению в
-- разные даты, поэтому рейсы хранятся отдельно от vehicle_destinations.
CREATE TABLE vehicle_trips (
    id               uuid PRIMARY KEY,
    vehicle_id       uuid NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    origin           jsonb NOT NULL,
    destination      jsonb NOT NULL,
    departure_date   date NOT NULL,
    loaded_weight_kg double precision NOT NULL DEFAULT 0 CHECK (loaded_weight_kg >= 0),
    loaded_volume_m3 double precision NOT NULL DEFAULT 0 CHECK (loaded_volume_m3 >= 0),
    status            text NOT NULL DEFAULT 'loading'
                      CHECK (status IN ('planned', 'loading', 'departed', 'completed')),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_vehicle_trips_vehicle_date
    ON vehicle_trips (vehicle_id, departure_date DESC, created_at DESC);
