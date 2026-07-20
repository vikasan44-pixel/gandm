DROP INDEX IF EXISTS idx_consolidation_suggestions_resolve;
ALTER TABLE consolidation_suggestions DROP COLUMN resolves_at;
