ALTER TABLE vehicle_trips
    DROP COLUMN IF EXISTS waypoints,
    DROP COLUMN IF EXISTS can_pickup_en_route;
