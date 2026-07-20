ALTER TABLE chats DROP CONSTRAINT chats_exactly_one_source;
DELETE FROM chats WHERE warehouse_offer_id IS NOT NULL;
ALTER TABLE chats DROP COLUMN warehouse_offer_id;
ALTER TABLE chats ADD CONSTRAINT chats_exactly_one_source
    CHECK (num_nonnulls(cargo_request_id, consolidated_request_id, driver_competition_id, warehouse_batch_id, transport_proposal_id) = 1);

DROP TABLE IF EXISTS warehouse_offers;
