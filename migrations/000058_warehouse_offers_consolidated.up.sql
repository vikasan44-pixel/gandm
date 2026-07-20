-- Phase 3: warehouses also bid on consolidated requests (a merged group of
-- nearby cargos). A warehouse offer now targets EITHER a single cargo request
-- OR a consolidated request — exactly one.
ALTER TABLE warehouse_offers ALTER COLUMN cargo_request_id DROP NOT NULL;
ALTER TABLE warehouse_offers ADD COLUMN consolidated_request_id uuid REFERENCES consolidated_requests (id) ON DELETE CASCADE;
ALTER TABLE warehouse_offers ADD CONSTRAINT warehouse_offers_one_target
    CHECK (num_nonnulls(cargo_request_id, consolidated_request_id) = 1);

-- Replace the single-cargo uniqueness with target-scoped partial uniques.
ALTER TABLE warehouse_offers DROP CONSTRAINT warehouse_offers_cargo_request_id_warehouse_id_key;
CREATE UNIQUE INDEX uq_warehouse_offer_cargo ON warehouse_offers (cargo_request_id, warehouse_id) WHERE cargo_request_id IS NOT NULL;
CREATE UNIQUE INDEX uq_warehouse_offer_cons ON warehouse_offers (consolidated_request_id, warehouse_id) WHERE consolidated_request_id IS NOT NULL;
CREATE INDEX idx_warehouse_offers_cons ON warehouse_offers (consolidated_request_id);
