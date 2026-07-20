ALTER TABLE vehicles
    DROP CONSTRAINT IF EXISTS vehicles_name_length,
    DROP COLUMN IF EXISTS name;
