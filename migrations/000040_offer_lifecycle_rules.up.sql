-- One participant may have only one active response per announcement.
-- Older accidental duplicates stay in history as withdrawn responses.
WITH ranked AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY cargo_request_id, consolidated_request_id, participant_id
               ORDER BY created_at DESC, id DESC
           ) AS position
    FROM offers
    WHERE status = 'submitted'
)
UPDATE offers
SET status = 'withdrawn'
FROM ranked
WHERE offers.id = ranked.id
  AND ranked.position > 1;

CREATE UNIQUE INDEX offers_one_submitted_per_cargo
    ON offers (cargo_request_id, participant_id)
    WHERE cargo_request_id IS NOT NULL AND status = 'submitted';

CREATE UNIQUE INDEX offers_one_submitted_per_consolidation
    ON offers (consolidated_request_id, participant_id)
    WHERE consolidated_request_id IS NOT NULL AND status = 'submitted';

ALTER TABLE consolidated_customs_offers
    DROP CONSTRAINT consolidated_customs_offers_status_check;
ALTER TABLE consolidated_customs_offers
    ADD CONSTRAINT consolidated_customs_offers_status_check
    CHECK (status IN ('submitted', 'selected', 'rejected', 'withdrawn'));

ALTER TABLE driver_competition_bids
    DROP CONSTRAINT driver_competition_bids_status_check;
ALTER TABLE driver_competition_bids
    ADD CONSTRAINT driver_competition_bids_status_check
    CHECK (status IN ('submitted', 'selected', 'rejected', 'withdrawn'));
