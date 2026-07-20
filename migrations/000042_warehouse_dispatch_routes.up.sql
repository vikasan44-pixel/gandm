ALTER TABLE warehouses
    ADD COLUMN dispatch_routes jsonb NOT NULL DEFAULT '[]'::jsonb;

-- Города забора, уже указанные пользователями, превращаем в рабочие
-- направления «склад → город». Так существующие данные сразу становятся
-- доступными в планах, а пользователю не приходится вводить их повторно.
INSERT INTO participant_routes (
    id, user_id,
    origin_lat, origin_lng, origin_label, origin_source, origin_country, origin_labels,
    destination_lat, destination_lng, destination_label, destination_source, destination_country, destination_labels,
    created_at
)
SELECT
    gen_random_uuid(), warehouse.user_id,
	CASE WHEN warehouse.address->>'lat' ~ '^[-+]?[0-9]+([.][0-9]+)?$' THEN (warehouse.address->>'lat')::numeric END,
	CASE WHEN warehouse.address->>'lng' ~ '^[-+]?[0-9]+([.][0-9]+)?$' THEN (warehouse.address->>'lng')::numeric END,
    warehouse.address->>'label',
	(CASE WHEN warehouse.address->>'source' IN ('osm', 'amap') THEN warehouse.address->>'source' ELSE 'osm' END)::coord_source,
    COALESCE(warehouse.address->>'country', ''),
    warehouse.address->'labels',
	CASE WHEN city.point->>'lat' ~ '^[-+]?[0-9]+([.][0-9]+)?$' THEN (city.point->>'lat')::numeric END,
	CASE WHEN city.point->>'lng' ~ '^[-+]?[0-9]+([.][0-9]+)?$' THEN (city.point->>'lng')::numeric END,
    city.point->>'label',
	(CASE WHEN city.point->>'source' IN ('osm', 'amap') THEN city.point->>'source' ELSE 'osm' END)::coord_source,
    COALESCE(city.point->>'country', ''),
    city.point->'labels',
    warehouse.created_at
FROM warehouses warehouse
CROSS JOIN LATERAL jsonb_array_elements(warehouse.pickup_cities) AS city(point)
WHERE warehouse.address->>'lat' ~ '^[-+]?[0-9]+([.][0-9]+)?$'
  AND warehouse.address->>'lng' ~ '^[-+]?[0-9]+([.][0-9]+)?$'
  AND city.point->>'lat' ~ '^[-+]?[0-9]+([.][0-9]+)?$'
  AND city.point->>'lng' ~ '^[-+]?[0-9]+([.][0-9]+)?$'
ON CONFLICT (user_id, origin_lat, origin_lng, destination_lat, destination_lng) DO NOTHING;

-- Сохраняем в каждом существующем складе все направления владельца,
-- включая только что созданные из городов забора и уже действующие планы.
UPDATE warehouses warehouse
SET dispatch_routes = COALESCE((
    SELECT jsonb_agg(
        jsonb_build_object(
            'id', route.id,
            'origin', jsonb_build_object(
                'lat', route.origin_lat,
                'lng', route.origin_lng,
                'label', route.origin_label,
                'source', route.origin_source,
                'country', route.origin_country,
                'labels', route.origin_labels
            ),
            'destination', jsonb_build_object(
                'lat', route.destination_lat,
                'lng', route.destination_lng,
                'label', route.destination_label,
                'source', route.destination_source,
                'country', route.destination_country,
                'labels', route.destination_labels
            )
        ) ORDER BY route.created_at
    )
    FROM participant_routes route
    WHERE route.user_id = warehouse.user_id
), '[]'::jsonb);
