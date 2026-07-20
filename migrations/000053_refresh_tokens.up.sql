-- Server-side registry of issued refresh tokens. Each refresh token carries a
-- jti (JWT ID) that must be present and un-revoked here to be accepted, which
-- makes refresh tokens revocable (logout, password change, account block) and
-- enables rotation with reuse-detection: presenting an already-revoked jti
-- means the token was replayed (likely stolen), so the whole subject's session
-- family is revoked.
CREATE TABLE refresh_tokens (
    jti          uuid PRIMARY KEY,
    subject_id   uuid NOT NULL,
    subject_type text NOT NULL CHECK (subject_type IN ('user', 'admin')),
    issued_at    timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz,
    -- The jti that superseded this one during rotation (audit / reuse trace).
    replaced_by  uuid
);

-- Revoke-all-for-subject and per-subject lookups.
CREATE INDEX idx_refresh_tokens_subject ON refresh_tokens (subject_type, subject_id);
