DROP INDEX IF EXISTS idx_dispatch_thresholds_warehouse_id;

ALTER TABLE warehouse_dispatch_thresholds
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS estimated_dispatch_date,
    DROP COLUMN IF EXISTS warehouse_id;

