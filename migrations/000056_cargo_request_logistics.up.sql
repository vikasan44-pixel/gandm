-- Richer logistics detail on a cargo request: packaging kind, number of
-- packages with per-package dimensions, whether it can be stacked, and whether
-- it needs ADR (dangerous-goods) transport. Carriers see these to price and
-- plan correctly.
ALTER TABLE cargo_requests
    -- 'packaged' (тарно-штучный, has discrete places) vs 'bulk' (россыпью).
    ADD COLUMN packaging    text NOT NULL DEFAULT 'packaged' CHECK (packaging IN ('packaged', 'bulk')),
    ADD COLUMN places_count integer NOT NULL DEFAULT 0,
    -- stackable: can other cargo be put on top; false = fragile / no load above.
    ADD COLUMN stackable    boolean NOT NULL DEFAULT true,
    -- adr_required: dangerous goods needing an ADR/ДОПОГ-certified driver+truck.
    ADD COLUMN adr_required boolean NOT NULL DEFAULT false;

-- One row per "место" (package) with its dimensions. Only used for packaged
-- cargo; bulk cargo has none.
CREATE TABLE cargo_request_items (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cargo_request_id uuid NOT NULL REFERENCES cargo_requests (id) ON DELETE CASCADE,
    position         integer NOT NULL DEFAULT 0,
    length_m         double precision NOT NULL DEFAULT 0,
    width_m          double precision NOT NULL DEFAULT 0,
    height_m         double precision NOT NULL DEFAULT 0
);

CREATE INDEX idx_cargo_request_items_cargo ON cargo_request_items (cargo_request_id);
