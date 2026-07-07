CREATE TABLE tools (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key         text NOT NULL UNIQUE,
    name        text NOT NULL,
    description text,
    category    text,
    is_active   boolean NOT NULL DEFAULT true
);
