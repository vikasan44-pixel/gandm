DROP TRIGGER IF EXISTS vehicle_active_trip_conflict_guard ON vehicle_trips;
DROP FUNCTION IF EXISTS prevent_multiple_active_vehicle_trips();

CREATE OR REPLACE FUNCTION prevent_vehicle_trip_date_conflict()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(
        hashtextextended(NEW.vehicle_id::text || ':' || NEW.departure_date::text, 0)
    );
    IF EXISTS (
        SELECT 1 FROM vehicle_trips trip
        WHERE trip.vehicle_id = NEW.vehicle_id
          AND trip.departure_date = NEW.departure_date
          AND trip.id <> NEW.id
    ) THEN
        RAISE EXCEPTION USING
            ERRCODE = '23505',
            CONSTRAINT = 'vehicle_trip_date_conflict',
            MESSAGE = 'vehicle already has a trip on this date';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER vehicle_trip_date_conflict_guard
BEFORE INSERT OR UPDATE OF vehicle_id, departure_date
ON vehicle_trips
FOR EACH ROW
EXECUTE FUNCTION prevent_vehicle_trip_date_conflict();
