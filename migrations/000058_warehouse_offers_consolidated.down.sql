DROP INDEX IF EXISTS idx_warehouse_offers_cons;
DROP INDEX IF EXISTS uq_warehouse_offer_cons;
DROP INDEX IF EXISTS uq_warehouse_offer_cargo;
ALTER TABLE warehouse_offers DROP CONSTRAINT IF EXISTS warehouse_offers_one_target;

-- Drop consolidated-only offers, then restore the single-cargo shape.
DELETE FROM warehouse_offers WHERE cargo_request_id IS NULL;
ALTER TABLE warehouse_offers DROP COLUMN consolidated_request_id;
ALTER TABLE warehouse_offers ALTER COLUMN cargo_request_id SET NOT NULL;
ALTER TABLE warehouse_offers ADD CONSTRAINT warehouse_offers_cargo_request_id_warehouse_id_key UNIQUE (cargo_request_id, warehouse_id);
