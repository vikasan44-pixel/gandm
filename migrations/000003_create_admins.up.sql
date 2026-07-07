CREATE TABLE admins (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    role          admin_role NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
