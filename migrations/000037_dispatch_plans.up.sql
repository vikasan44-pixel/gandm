-- План консолидации склада по направлению.
-- Физическая площадь склада остаётся в warehouses (м²), а здесь хранится
-- объём партии для ближайшей отправки (м³).

ALTER TABLE warehouse_dispatch_thresholds
    ADD COLUMN warehouse_id uuid REFERENCES warehouses (id) ON DELETE CASCADE,
    ADD COLUMN estimated_dispatch_date date,
    ADD COLUMN status text NOT NULL DEFAULT 'collecting'
        CHECK (status IN ('collecting', 'ready', 'paused', 'dispatched'));

-- Старые пороги аккуратно связываем с первым складом владельца, если склад
-- уже создан. Если склада ещё нет, порог продолжит работать как раньше и
-- его можно будет привязать при следующем редактировании.
UPDATE warehouse_dispatch_thresholds threshold
SET warehouse_id = (
    SELECT warehouse.id
    FROM participant_routes route
    JOIN warehouses warehouse ON warehouse.user_id = route.user_id
    WHERE route.id = threshold.route_id
    ORDER BY warehouse.created_at ASC
    LIMIT 1
)
WHERE threshold.warehouse_id IS NULL;

CREATE INDEX idx_dispatch_thresholds_warehouse_id
    ON warehouse_dispatch_thresholds (warehouse_id);
