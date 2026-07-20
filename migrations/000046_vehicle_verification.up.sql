ALTER TABLE vehicles
    ADD COLUMN registration_country text NOT NULL DEFAULT '',
    ADD COLUMN plate_number text NOT NULL DEFAULT '',
    ADD COLUMN vin text NOT NULL DEFAULT '',
    ADD COLUMN verification_status text NOT NULL DEFAULT 'not_submitted'
        CHECK (verification_status IN ('not_submitted', 'pending', 'verified', 'rejected')),
    ADD COLUMN privacy_consent_at timestamptz,
    ADD COLUMN privacy_consent_version text NOT NULL DEFAULT '',
    ADD COLUMN verified_at timestamptz,
    ADD COLUMN verified_by uuid REFERENCES admins (id) ON DELETE SET NULL,
    ADD COLUMN verification_reject_reason text;

CREATE UNIQUE INDEX idx_vehicles_country_plate_unique
    ON vehicles (lower(registration_country), lower(plate_number))
    WHERE plate_number <> '';

CREATE UNIQUE INDEX idx_vehicles_vin_unique
    ON vehicles (upper(vin))
    WHERE vin <> '';

CREATE TABLE vehicle_documents (
    id            uuid PRIMARY KEY,
    vehicle_id    uuid NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    type          text NOT NULL CHECK (type IN (
        'registration_certificate', 'identity_document', 'insurance',
        'photo_front', 'photo_back', 'photo_left', 'photo_right'
    )),
    file_url      text NOT NULL,
    original_name text NOT NULL,
    content_type  text NOT NULL,
    uploaded_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (vehicle_id, type)
);

CREATE INDEX idx_vehicle_documents_vehicle ON vehicle_documents (vehicle_id);
CREATE INDEX idx_vehicles_verification_queue ON vehicles (verification_status, created_at);
