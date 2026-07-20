-- A vehicle may keep completed trips as history, but it can have only one
-- active direction (planned/loading/departed) at any moment.
DROP TRIGGER IF EXISTS vehicle_trip_date_conflict_guard ON vehicle_trips;
DROP FUNCTION IF EXISTS prevent_vehicle_trip_date_conflict();

CREATE OR REPLACE FUNCTION prevent_multiple_active_vehicle_trips()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    -- Serialize all active-trip changes for the same vehicle.
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.vehicle_id::text, 0));

    IF NEW.status <> 'completed' AND EXISTS (
        SELECT 1
        FROM vehicle_trips trip
        WHERE trip.vehicle_id = NEW.vehicle_id
          AND trip.status <> 'completed'
          AND trip.id <> NEW.id
    ) THEN
        RAISE EXCEPTION USING
            ERRCODE = '23505',
            CONSTRAINT = 'vehicle_active_trip_conflict',
            MESSAGE = 'vehicle already has an active trip';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER vehicle_active_trip_conflict_guard
BEFORE INSERT OR UPDATE
ON vehicle_trips
FOR EACH ROW
EXECUTE FUNCTION prevent_multiple_active_vehicle_trips();
