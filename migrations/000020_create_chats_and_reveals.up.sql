CREATE TABLE contact_reveals (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id        uuid NOT NULL REFERENCES users (id),
    participant_id   uuid NOT NULL REFERENCES users (id),
    cargo_request_id uuid NOT NULL REFERENCES cargo_requests (id) ON DELETE CASCADE,
    is_paid          boolean NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (client_id, participant_id, cargo_request_id)
);

CREATE INDEX idx_contact_reveals_client_id ON contact_reveals (client_id);

CREATE TABLE chats (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cargo_request_id uuid NOT NULL REFERENCES cargo_requests (id) ON DELETE CASCADE,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_chats_cargo_request_id ON chats (cargo_request_id);

CREATE TABLE chat_participants (
    chat_id uuid NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX idx_chat_participants_user_id ON chat_participants (user_id);

CREATE TABLE messages (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id        uuid NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    sender_id      uuid NOT NULL REFERENCES users (id),
    body           text NOT NULL,
    attachment_url text,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_messages_chat_created ON messages (chat_id, created_at);
