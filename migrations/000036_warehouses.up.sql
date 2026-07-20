CREATE TABLE warehouses (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    address jsonb NOT NULL,
    contact_name text NOT NULL DEFAULT '',
    contact_phone text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    work_hours text NOT NULL DEFAULT '',
    covered_area_m2 double precision NOT NULL DEFAULT 0 CHECK (covered_area_m2 >= 0),
    open_area_m2 double precision NOT NULL DEFAULT 0 CHECK (open_area_m2 >= 0),
    available_covered_area_m2 double precision NOT NULL DEFAULT 0 CHECK (available_covered_area_m2 >= 0),
    available_open_area_m2 double precision NOT NULL DEFAULT 0 CHECK (available_open_area_m2 >= 0),
    max_weight_kg double precision NOT NULL DEFAULT 0 CHECK (max_weight_kg >= 0),
    max_volume_m3 double precision NOT NULL DEFAULT 0 CHECK (max_volume_m3 >= 0),
    services text[] NOT NULL DEFAULT '{}',
    consolidation_enabled boolean NOT NULL DEFAULT false,
    consolidation_min_volume_m3 double precision NOT NULL DEFAULT 0,
    consolidation_frequency text NOT NULL DEFAULT '',
    pickup_enabled boolean NOT NULL DEFAULT false,
    pickup_cities jsonb NOT NULL DEFAULT '[]',
    pickup_radius_km double precision NOT NULL DEFAULT 0,
    own_transport boolean NOT NULL DEFAULT false,
    pickup_max_weight_kg double precision NOT NULL DEFAULT 0,
    pickup_max_volume_m3 double precision NOT NULL DEFAULT 0,
    pickup_price_mode text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'paused')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_warehouses_user_id ON warehouses(user_id);
CREATE INDEX idx_warehouses_status ON warehouses(status);
