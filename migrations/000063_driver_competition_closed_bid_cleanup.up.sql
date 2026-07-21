-- A closed competition must not retain actionable submitted bids. This also
-- normalizes bids belonging to duplicate competitions closed by migration 62.
UPDATE driver_competition_bids AS bid
SET status = 'rejected'
FROM driver_competitions AS competition
WHERE bid.competition_id = competition.id
  AND competition.status = 'closed'
  AND bid.status = 'submitted';
