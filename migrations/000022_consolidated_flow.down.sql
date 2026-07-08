DROP TABLE IF EXISTS consolidated_selections;
DROP TABLE IF EXISTS consolidated_payments;

-- Consolidation chats can't exist under the old NOT NULL schema.
DELETE FROM chats WHERE cargo_request_id IS NULL;
ALTER TABLE chats DROP CONSTRAINT IF EXISTS chats_exactly_one_source;
ALTER TABLE chats DROP COLUMN IF EXISTS consolidated_request_id;
ALTER TABLE chats ALTER COLUMN cargo_request_id SET NOT NULL;

ALTER TABLE consolidated_requests
    DROP COLUMN IF EXISTS chat_id,
    DROP COLUMN IF EXISTS invited_client_id,
    DROP COLUMN IF EXISTS initiator_client_id,
    DROP COLUMN IF EXISTS invite_status;

DROP TYPE IF EXISTS consolidated_invite_status;
