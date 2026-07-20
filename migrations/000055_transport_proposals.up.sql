-- Direct price negotiation between a cargo owner and a specific vehicle's
-- carrier, initiated from transport search. The client sends cargo details to
-- the carrier and they haggle over a price; on agreement contacts are revealed
-- and a chat opens. Cargo is either an inline snapshot (typed in the form) or
-- copied from one of the client's existing cargo requests.
CREATE TABLE transport_proposals (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id           uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    vehicle_id          uuid NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    carrier_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- Set when the client picked one of their existing cargo requests; NULL for
    -- an ad-hoc cargo typed straight into the proposal form.
    cargo_request_id    uuid REFERENCES cargo_requests (id) ON DELETE SET NULL,

    origin_lat          double precision NOT NULL,
    origin_lng          double precision NOT NULL,
    origin_label        text NOT NULL DEFAULT '',
    origin_source       text NOT NULL DEFAULT 'osm',
    origin_country      text NOT NULL DEFAULT '',
    origin_labels       jsonb,
    destination_lat     double precision NOT NULL,
    destination_lng     double precision NOT NULL,
    destination_label   text NOT NULL DEFAULT '',
    destination_source  text NOT NULL DEFAULT 'osm',
    destination_country text NOT NULL DEFAULT '',
    destination_labels  jsonb,

    cargo_name          text NOT NULL DEFAULT '',
    volume_m3           double precision NOT NULL DEFAULT 0,
    weight_kg           double precision NOT NULL DEFAULT 0,
    places_count        integer NOT NULL DEFAULT 0,
    pickup_date         text NOT NULL DEFAULT '',

    -- Negotiation state machine:
    -- sent → carrier_quoted → client_countered → carrier_final → agreed|rejected
    status              text NOT NULL DEFAULT 'sent'
        CHECK (status IN ('sent', 'carrier_quoted', 'client_countered', 'carrier_final', 'agreed', 'rejected')),
    current_price       numeric,
    last_price_by       text CHECK (last_price_by IN ('carrier', 'client')),
    currency            text NOT NULL DEFAULT 'KZT',

    chat_id             uuid REFERENCES chats (id) ON DELETE SET NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_transport_proposals_client ON transport_proposals (client_id);
CREATE INDEX idx_transport_proposals_carrier ON transport_proposals (carrier_id);

-- One row per "место" (package): its dimensions.
CREATE TABLE transport_proposal_items (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    proposal_id uuid NOT NULL REFERENCES transport_proposals (id) ON DELETE CASCADE,
    position    integer NOT NULL DEFAULT 0,
    length_m    double precision NOT NULL DEFAULT 0,
    width_m     double precision NOT NULL DEFAULT 0,
    height_m    double precision NOT NULL DEFAULT 0
);

CREATE INDEX idx_transport_proposal_items_proposal ON transport_proposal_items (proposal_id);

-- A proposal that ends in agreement opens a chat — add it as a fifth chat
-- context and extend the exactly-one-source guard.
ALTER TABLE chats ADD COLUMN transport_proposal_id uuid REFERENCES transport_proposals (id) ON DELETE CASCADE;
ALTER TABLE chats DROP CONSTRAINT chats_exactly_one_source;
ALTER TABLE chats ADD CONSTRAINT chats_exactly_one_source
    CHECK (num_nonnulls(cargo_request_id, consolidated_request_id, driver_competition_id, warehouse_batch_id, transport_proposal_id) = 1);
