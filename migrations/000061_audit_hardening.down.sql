UPDATE tools SET is_active = true
WHERE key IN ('create_cargo_request', 'view_cargo_requests', 'submit_fill_report');

DROP INDEX IF EXISTS idx_warehouse_offers_one_selected_consolidated;
DROP INDEX IF EXISTS idx_warehouse_offers_one_selected_cargo;
ALTER TABLE currency_rates DROP CONSTRAINT IF EXISTS currency_rates_positive;
ALTER TABLE warehouse_offers DROP CONSTRAINT IF EXISTS warehouse_offers_price_positive;
ALTER TABLE transport_proposal_items
    DROP CONSTRAINT IF EXISTS transport_proposal_items_position_nonnegative,
    DROP CONSTRAINT IF EXISTS transport_proposal_items_positive_dimensions;
ALTER TABLE transport_proposals
    DROP CONSTRAINT IF EXISTS transport_proposals_price_positive,
    DROP CONSTRAINT IF EXISTS transport_proposals_places_nonnegative,
    DROP CONSTRAINT IF EXISTS transport_proposals_positive_load;
DROP INDEX IF EXISTS idx_refresh_tokens_expires_at;
DROP TABLE IF EXISTS warehouse_dispatch_route_links;
ALTER TABLE participant_routes DROP COLUMN IF EXISTS route_source;
ALTER TABLE users DROP COLUMN IF EXISTS legal_form;
