DROP INDEX IF EXISTS offers_one_submitted_per_consolidation;
DROP INDEX IF EXISTS offers_one_submitted_per_cargo;

UPDATE offers SET status = 'rejected' WHERE status = 'withdrawn';
UPDATE consolidated_customs_offers SET status = 'rejected' WHERE status = 'withdrawn';
UPDATE driver_competition_bids SET status = 'rejected' WHERE status = 'withdrawn';

ALTER TABLE consolidated_customs_offers
    DROP CONSTRAINT consolidated_customs_offers_status_check;
ALTER TABLE consolidated_customs_offers
    ADD CONSTRAINT consolidated_customs_offers_status_check
    CHECK (status IN ('submitted', 'selected', 'rejected'));

ALTER TABLE driver_competition_bids
    DROP CONSTRAINT driver_competition_bids_status_check;
ALTER TABLE driver_competition_bids
    ADD CONSTRAINT driver_competition_bids_status_check
    CHECK (status IN ('submitted', 'selected', 'rejected'));
