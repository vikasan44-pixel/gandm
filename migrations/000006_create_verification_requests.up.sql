CREATE TABLE verification_requests (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status        verification_status NOT NULL DEFAULT 'pending',
    reject_reason text,
    reviewed_by   uuid REFERENCES admins (id),
    reviewed_at   timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_verification_requests_user_id ON verification_requests (user_id);
CREATE INDEX idx_verification_requests_status_created_at ON verification_requests (status, created_at);
