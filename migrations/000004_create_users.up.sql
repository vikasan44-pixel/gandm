CREATE TABLE users (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email            text NOT NULL UNIQUE,
    phone            text,
    company_name     text,
    participant_type participant_type NOT NULL,
    password_hash    text NOT NULL,
    status           user_status NOT NULL DEFAULT 'pending',
    has_subscription boolean NOT NULL DEFAULT false,
    language         text NOT NULL DEFAULT 'ru',
    created_at       timestamptz NOT NULL DEFAULT now(),
    last_active_at   timestamptz
);

CREATE INDEX idx_users_status ON users (status);
CREATE INDEX idx_users_participant_type ON users (participant_type);
CREATE INDEX idx_users_company_name ON users (company_name);
