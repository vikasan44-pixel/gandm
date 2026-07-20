DROP TRIGGER IF EXISTS vehicle_trip_date_conflict_guard ON vehicle_trips;
DROP FUNCTION IF EXISTS prevent_vehicle_trip_date_conflict();
