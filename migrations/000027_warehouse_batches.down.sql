ALTER TABLE chats DROP CONSTRAINT chats_exactly_one_source;
DELETE FROM chats WHERE warehouse_batch_id IS NOT NULL;
ALTER TABLE chats DROP COLUMN warehouse_batch_id;
ALTER TABLE chats ADD CONSTRAINT chats_exactly_one_source
    CHECK (num_nonnulls(cargo_request_id, consolidated_request_id, driver_competition_id) = 1);

DROP TABLE IF EXISTS warehouse_batch_members;
DROP TABLE IF EXISTS warehouse_batches;
