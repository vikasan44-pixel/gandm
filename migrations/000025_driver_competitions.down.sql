ALTER TABLE chats DROP CONSTRAINT chats_exactly_one_source;
DELETE FROM chats WHERE driver_competition_id IS NOT NULL;
ALTER TABLE chats DROP COLUMN driver_competition_id;
ALTER TABLE chats ADD CONSTRAINT chats_exactly_one_source
    CHECK (num_nonnulls(cargo_request_id, consolidated_request_id) = 1);

DROP TABLE IF EXISTS driver_competition_bids;
DROP TABLE IF EXISTS driver_competitions;
