-- Full consolidation flow: paid invite/accept between the two clients, the
-- shared client chat, and joint carrier selection.

CREATE TYPE consolidated_invite_status AS ENUM ('none', 'invited', 'accepted');

ALTER TABLE consolidated_requests
    ADD COLUMN invite_status       consolidated_invite_status NOT NULL DEFAULT 'none',
    ADD COLUMN initiator_client_id uuid REFERENCES users (id),
    ADD COLUMN invited_client_id   uuid REFERENCES users (id),
    ADD COLUMN chat_id             uuid REFERENCES chats (id);

-- A chat now belongs to either a single cargo request (Stage 3 select flow)
-- or a consolidated request (shared client chat) — exactly one.
ALTER TABLE chats ALTER COLUMN cargo_request_id DROP NOT NULL;
ALTER TABLE chats ADD COLUMN consolidated_request_id uuid REFERENCES consolidated_requests (id) ON DELETE CASCADE;
ALTER TABLE chats ADD CONSTRAINT chats_exactly_one_source
    CHECK (num_nonnulls(cargo_request_id, consolidated_request_id) = 1);

-- One-time sandbox/manual payments unlocking a single consolidation for the
-- invited client (subscription unlocks all of them).
CREATE TABLE consolidated_payments (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    consolidated_request_id uuid NOT NULL REFERENCES consolidated_requests (id) ON DELETE CASCADE,
    client_id               uuid NOT NULL REFERENCES users (id),
    provider                text NOT NULL,
    provider_ref            text NOT NULL,
    created_at              timestamptz NOT NULL DEFAULT now(),
    UNIQUE (consolidated_request_id, client_id)
);

-- Each client's current carrier choice; re-selectable until the deal closes.
CREATE TABLE consolidated_selections (
    consolidated_request_id uuid NOT NULL REFERENCES consolidated_requests (id) ON DELETE CASCADE,
    client_id               uuid NOT NULL REFERENCES users (id),
    offer_id                uuid NOT NULL REFERENCES offers (id) ON DELETE CASCADE,
    created_at              timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (consolidated_request_id, client_id)
);
