DROP TABLE IF EXISTS vehicle_documents;
DROP INDEX IF EXISTS idx_vehicles_verification_queue;
DROP INDEX IF EXISTS idx_vehicles_vin_unique;
DROP INDEX IF EXISTS idx_vehicles_country_plate_unique;

ALTER TABLE vehicles
    DROP COLUMN IF EXISTS verification_reject_reason,
    DROP COLUMN IF EXISTS verified_by,
    DROP COLUMN IF EXISTS verified_at,
    DROP COLUMN IF EXISTS privacy_consent_version,
    DROP COLUMN IF EXISTS privacy_consent_at,
    DROP COLUMN IF EXISTS verification_status,
    DROP COLUMN IF EXISTS vin,
    DROP COLUMN IF EXISTS plate_number,
    DROP COLUMN IF EXISTS registration_country;
