DROP TRIGGER IF EXISTS vehicle_active_trip_conflict_guard ON vehicle_trips;
DROP FUNCTION IF EXISTS prevent_multiple_active_vehicle_trips();

-- A vehicle may publish several zero-load planned directions. As soon as one
-- direction receives cargo (load > 0) or starts loading/in transit, only that
-- direction remains active until it is completed or deleted.
CREATE OR REPLACE FUNCTION prevent_multiple_committed_vehicle_trips()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.vehicle_id::text, 0));

    IF (
        NEW.status IN ('loading', 'departed')
        OR (NEW.status = 'planned' AND (NEW.loaded_weight_kg > 0 OR NEW.loaded_volume_m3 > 0))
    ) AND EXISTS (
        SELECT 1
        FROM vehicle_trips trip
        WHERE trip.vehicle_id = NEW.vehicle_id
          AND trip.id <> NEW.id
          AND (
              trip.status IN ('loading', 'departed')
              OR (trip.status = 'planned' AND (trip.loaded_weight_kg > 0 OR trip.loaded_volume_m3 > 0))
          )
    ) THEN
        RAISE EXCEPTION USING
            ERRCODE = '23505',
            CONSTRAINT = 'vehicle_active_trip_conflict',
            MESSAGE = 'vehicle already has a direction with active cargo';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER vehicle_active_trip_conflict_guard
BEFORE INSERT OR UPDATE
ON vehicle_trips
FOR EACH ROW
EXECUTE FUNCTION prevent_multiple_committed_vehicle_trips();
