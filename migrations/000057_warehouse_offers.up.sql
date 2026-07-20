-- Phase 2: warehouses bid on a cargo request. A published, pickup-enabled
-- warehouse whose address is within its pickup_radius_km of the cargo origin is
-- notified and can propose a price to collect/consolidate/dispatch the cargo.
-- The client picks one warehouse offer, which reveals the warehouse contact and
-- opens a shared chat.
CREATE TABLE warehouse_offers (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cargo_request_id   uuid NOT NULL REFERENCES cargo_requests (id) ON DELETE CASCADE,
    warehouse_id       uuid NOT NULL REFERENCES warehouses (id) ON DELETE CASCADE,
    warehouse_owner_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    price              numeric NOT NULL,
    currency           text NOT NULL DEFAULT 'KZT',
    conditions         text NOT NULL DEFAULT '',
    status             text NOT NULL DEFAULT 'submitted'
        CHECK (status IN ('submitted', 'selected', 'rejected')),
    chat_id            uuid REFERENCES chats (id) ON DELETE SET NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    -- One offer per warehouse per cargo request.
    UNIQUE (cargo_request_id, warehouse_id)
);

CREATE INDEX idx_warehouse_offers_cargo ON warehouse_offers (cargo_request_id);
CREATE INDEX idx_warehouse_offers_owner ON warehouse_offers (warehouse_owner_id);

-- A selected warehouse offer opens a chat — add it as a sixth chat context.
ALTER TABLE chats ADD COLUMN warehouse_offer_id uuid REFERENCES warehouse_offers (id) ON DELETE CASCADE;
ALTER TABLE chats DROP CONSTRAINT chats_exactly_one_source;
ALTER TABLE chats ADD CONSTRAINT chats_exactly_one_source
    CHECK (num_nonnulls(cargo_request_id, consolidated_request_id, driver_competition_id, warehouse_batch_id, transport_proposal_id, warehouse_offer_id) = 1);
