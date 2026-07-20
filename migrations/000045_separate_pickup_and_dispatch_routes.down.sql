-- Откат восстанавливает старое поведение для существующих данных: добавляет
-- направления «склад → город забора», используя сохранённые participant_routes.
UPDATE warehouses warehouse
SET dispatch_routes = warehouse.dispatch_routes || COALESCE((
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
        )
    )
    FROM participant_routes route
    CROSS JOIN LATERAL jsonb_array_elements(warehouse.pickup_cities) AS city(point)
    WHERE route.user_id = warehouse.user_id
      AND route.origin_lat = (warehouse.address->>'lat')::numeric
      AND route.origin_lng = (warehouse.address->>'lng')::numeric
      AND route.destination_lat = (city.point->>'lat')::numeric
      AND route.destination_lng = (city.point->>'lng')::numeric
      AND NOT EXISTS (
          SELECT 1
          FROM jsonb_array_elements(warehouse.dispatch_routes) existing(item)
          WHERE existing.item->>'id' = route.id::text
      )
), '[]'::jsonb);
