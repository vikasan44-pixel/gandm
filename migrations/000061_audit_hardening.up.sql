-- Identity profile. Existing accounts were registered with a mandatory
-- company name, so preserve them as legal entities; new accounts default to
-- individuals unless the registration payload says otherwise.
ALTER TABLE users
    ADD COLUMN legal_form text NOT NULL DEFAULT 'legal_entity'
        CHECK (legal_form IN ('individual', 'legal_entity'));
ALTER TABLE users ALTER COLUMN legal_form SET DEFAULT 'individual';

-- Registry cleanup and lookup performance.
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens (expires_at);

-- Explicit links keep the warehouse JSON snapshot and the shared route table
-- synchronized. Only routes created by a warehouse may be cleaned up by a
-- warehouse edit; manually configured participant routes are preserved.
ALTER TABLE participant_routes
    ADD COLUMN route_source text NOT NULL DEFAULT 'manual'
        CHECK (route_source IN ('manual', 'warehouse'));
CREATE TABLE warehouse_dispatch_route_links (
    warehouse_id uuid NOT NULL REFERENCES warehouses (id) ON DELETE CASCADE,
    route_id     uuid NOT NULL REFERENCES participant_routes (id) ON DELETE CASCADE,
    PRIMARY KEY (warehouse_id, route_id)
);
INSERT INTO warehouse_dispatch_route_links (warehouse_id, route_id)
SELECT warehouse.id, route.id
FROM warehouses warehouse
CROSS JOIN LATERAL jsonb_array_elements(warehouse.dispatch_routes) item
JOIN participant_routes route
  ON route.id = CASE
       WHEN item->>'id' ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
       THEN (item->>'id')::uuid END
ON CONFLICT DO NOTHING;

-- Business invariants remain true even if an API path is added later without
-- service validation.
ALTER TABLE transport_proposals
    ADD CONSTRAINT transport_proposals_positive_load CHECK (volume_m3 > 0 AND weight_kg > 0),
    ADD CONSTRAINT transport_proposals_places_nonnegative CHECK (places_count >= 0),
    ADD CONSTRAINT transport_proposals_price_positive CHECK (current_price IS NULL OR current_price > 0);
ALTER TABLE transport_proposal_items
    ADD CONSTRAINT transport_proposal_items_positive_dimensions CHECK (length_m > 0 AND width_m > 0 AND height_m > 0),
    ADD CONSTRAINT transport_proposal_items_position_nonnegative CHECK (position >= 0);
ALTER TABLE warehouse_offers
    ADD CONSTRAINT warehouse_offers_price_positive CHECK (price > 0);
ALTER TABLE currency_rates
    ADD CONSTRAINT currency_rates_positive CHECK (kzt_per_unit > 0);

CREATE UNIQUE INDEX idx_warehouse_offers_one_selected_cargo
    ON warehouse_offers (cargo_request_id)
    WHERE status = 'selected' AND cargo_request_id IS NOT NULL;
CREATE UNIQUE INDEX idx_warehouse_offers_one_selected_consolidated
    ON warehouse_offers (consolidated_request_id)
    WHERE status = 'selected' AND consolidated_request_id IS NOT NULL;

-- These historical placeholders are not real paid tools. The fill-report tool
-- was retired by migration 44 and must stay retired even after seed reruns.
UPDATE tools
SET is_active = false
WHERE key IN ('create_cargo_request', 'view_cargo_requests', 'submit_fill_report');
