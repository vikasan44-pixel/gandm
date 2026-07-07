-- Haversine distance in kilometers between two WGS-84 points, for radius
-- matching in SQL. Must stay consistent with geo.HaversineKm in Go
-- (internal/geo/geo.go) — both use R = 6371 km. least(1.0, ...) clamps
-- floating-point drift before asin.
CREATE FUNCTION haversine_km(
    lat1 double precision, lng1 double precision,
    lat2 double precision, lng2 double precision
) RETURNS double precision
LANGUAGE sql IMMUTABLE
AS $$
    SELECT 6371.0 * 2 * asin(
        least(1.0, sqrt(
            power(sin(radians(lat2 - lat1) / 2), 2) +
            cos(radians(lat1)) * cos(radians(lat2)) * power(sin(radians(lng2 - lng1) / 2), 2)
        ))
    )
$$;
