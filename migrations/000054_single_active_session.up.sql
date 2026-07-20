-- One active login per participant/admin account. Access and refresh JWTs
-- carry session_id; middleware compares it with this row on every request.
CREATE TABLE active_sessions (
    subject_type text NOT NULL CHECK (subject_type IN ('user', 'admin')),
    subject_id   uuid NOT NULL,
    session_id   uuid NOT NULL,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (subject_type, subject_id)
);

CREATE UNIQUE INDEX idx_active_sessions_session_id ON active_sessions (session_id);
