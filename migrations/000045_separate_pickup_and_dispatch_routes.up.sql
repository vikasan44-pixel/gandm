-- До исправления каждый город забора автоматически превращался в направление
-- отправки «адрес склада → город забора». Удаляем такие исторически созданные
-- направления, только если на них нет действующего плана отправки.
UPDATE warehouses warehouse
SET dispatch_routes = COALESCE((
    SELECT jsonb_agg(route.item)
    FROM jsonb_array_elements(warehouse.dispatch_routes) AS route(item)
    WHERE NOT (
        NOT EXISTS (
            SELECT 1
            FROM warehouse_dispatch_thresholds threshold
			WHERE threshold.route_id = CASE
				WHEN route.item->>'id' ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
				THEN (route.item->>'id')::uuid END
		)
		AND CASE WHEN route.item->'origin'->>'lat' ~ '^[-+]?[0-9]+([.][0-9]+)?$' THEN (route.item->'origin'->>'lat')::numeric END
			= CASE WHEN warehouse.address->>'lat' ~ '^[-+]?[0-9]+([.][0-9]+)?$' THEN (warehouse.address->>'lat')::numeric END
		AND CASE WHEN route.item->'origin'->>'lng' ~ '^[-+]?[0-9]+([.][0-9]+)?$' THEN (route.item->'origin'->>'lng')::numeric END
			= CASE WHEN warehouse.address->>'lng' ~ '^[-+]?[0-9]+([.][0-9]+)?$' THEN (warehouse.address->>'lng')::numeric END
        AND EXISTS (
            SELECT 1
            FROM jsonb_array_elements(warehouse.pickup_cities) AS city(point)
			WHERE CASE WHEN city.point->>'lat' ~ '^[-+]?[0-9]+([.][0-9]+)?$' THEN (city.point->>'lat')::numeric END
				= CASE WHEN route.item->'destination'->>'lat' ~ '^[-+]?[0-9]+([.][0-9]+)?$' THEN (route.item->'destination'->>'lat')::numeric END
			  AND CASE WHEN city.point->>'lng' ~ '^[-+]?[0-9]+([.][0-9]+)?$' THEN (city.point->>'lng')::numeric END
				= CASE WHEN route.item->'destination'->>'lng' ~ '^[-+]?[0-9]+([.][0-9]+)?$' THEN (route.item->'destination'->>'lng')::numeric END
        )
    )
), '[]'::jsonb);
