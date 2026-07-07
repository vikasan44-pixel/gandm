-- Offers created against consolidated requests can't survive the rollback.
DELETE FROM offers WHERE consolidated_request_id IS NOT NULL;

ALTER TABLE offers DROP CONSTRAINT IF EXISTS offers_exactly_one_target;
ALTER TABLE offers DROP COLUMN IF EXISTS consolidated_request_id;
ALTER TABLE offers ALTER COLUMN cargo_request_id SET NOT NULL;

DROP TABLE IF EXISTS consolidated_requests;
DROP TABLE IF EXISTS consolidation_suggestions;
DROP TYPE IF EXISTS consolidation_status;
DROP TABLE IF EXISTS platform_settings;
