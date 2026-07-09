-- Конкурс водителей (ТЗ §11.4): склад объявляет конкурс на перевозку по
-- своему направлению; водители (инструмент manage_fleet + подходящий
-- маршрут) подают ценовые предложения; склад выбирает по цене и рейтингу,
-- водитель подключается к чату (§11.3).

CREATE TABLE driver_competitions (
    id            uuid PRIMARY KEY,
    warehouse_id  uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    route_id      uuid NOT NULL REFERENCES participant_routes (id) ON DELETE CASCADE,
    volume_m3     numeric NOT NULL CHECK (volume_m3 > 0),
    -- Примерная дата отправки — свободный текст («середина июля», дата…):
    -- на этом этапе это ориентир для водителя, не планировочное поле.
    dispatch_date text NOT NULL DEFAULT '',
    status        text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_driver_competitions_warehouse ON driver_competitions (warehouse_id);
CREATE INDEX idx_driver_competitions_status ON driver_competitions (status);

CREATE TABLE driver_competition_bids (
    id             uuid PRIMARY KEY,
    competition_id uuid NOT NULL REFERENCES driver_competitions (id) ON DELETE CASCADE,
    driver_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    price          numeric NOT NULL CHECK (price > 0),
    currency       text NOT NULL DEFAULT 'KZT',
    comment        text NOT NULL DEFAULT '',
    status         text NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted', 'selected', 'rejected')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (competition_id, driver_id)
);

CREATE INDEX idx_driver_bids_competition ON driver_competition_bids (competition_id);

-- Чат «склад + выбранный водитель»: третий взаимоисключающий источник чата.
ALTER TABLE chats ADD COLUMN driver_competition_id uuid REFERENCES driver_competitions (id) ON DELETE CASCADE;
ALTER TABLE chats DROP CONSTRAINT chats_exactly_one_source;
ALTER TABLE chats ADD CONSTRAINT chats_exactly_one_source
    CHECK (num_nonnulls(cargo_request_id, consolidated_request_id, driver_competition_id) = 1);
