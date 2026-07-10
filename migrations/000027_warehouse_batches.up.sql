-- Партия склада (ТЗ §10.1): порог отправки достигнут → грузы едут вместе →
-- общий чат всех клиентов партии + склада с напоминанием про документы.
-- Партия «активна», пока склад не сбросит набранный объём ниже порога
-- (отправил партию) — тогда dispatched_at проставляется и следующее
-- достижение порога открывает новую партию.

CREATE TABLE warehouse_batches (
    id            uuid PRIMARY KEY,
    warehouse_id  uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    route_id      uuid NOT NULL REFERENCES participant_routes (id) ON DELETE CASCADE,
    volume_m3     numeric NOT NULL CHECK (volume_m3 > 0),
    dispatched_at timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_warehouse_batches_route ON warehouse_batches (route_id);

-- Какие грузы едут в партии — фиксируется на момент открытия чата.
CREATE TABLE warehouse_batch_members (
    batch_id         uuid NOT NULL REFERENCES warehouse_batches (id) ON DELETE CASCADE,
    cargo_request_id uuid NOT NULL REFERENCES cargo_requests (id) ON DELETE CASCADE,
    client_id        uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (batch_id, cargo_request_id)
);

-- Чат партии — четвёртый взаимоисключающий источник чата.
ALTER TABLE chats ADD COLUMN warehouse_batch_id uuid REFERENCES warehouse_batches (id) ON DELETE CASCADE;
ALTER TABLE chats DROP CONSTRAINT chats_exactly_one_source;
ALTER TABLE chats ADD CONSTRAINT chats_exactly_one_source
    CHECK (num_nonnulls(cargo_request_id, consolidated_request_id, driver_competition_id, warehouse_batch_id) = 1);
