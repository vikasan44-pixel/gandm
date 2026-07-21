-- Keep at most one active driver competition for a participant route. Close
-- historical duplicates deterministically before installing the invariant so
-- existing environments can migrate safely.
WITH ranked AS (
    SELECT id,
           row_number() OVER (PARTITION BY route_id ORDER BY created_at DESC, id DESC) AS position
    FROM driver_competitions
    WHERE status = 'open'
)
UPDATE driver_competitions AS competition
SET status = 'closed'
FROM ranked
WHERE competition.id = ranked.id
  AND ranked.position > 1;

CREATE UNIQUE INDEX idx_driver_competitions_one_open_per_route
    ON driver_competitions (route_id)
    WHERE status = 'open';
