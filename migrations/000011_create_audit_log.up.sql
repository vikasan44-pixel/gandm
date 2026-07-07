CREATE TABLE audit_log (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id       uuid NOT NULL REFERENCES admins (id),
    action         text NOT NULL,
    target_user_id uuid REFERENCES users (id),
    details        jsonb,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_admin_id ON audit_log (admin_id);
CREATE INDEX idx_audit_log_target_user_id ON audit_log (target_user_id);
CREATE INDEX idx_audit_log_created_at ON audit_log (created_at);
