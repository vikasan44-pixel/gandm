-- Stage 7: real functionality behind the three placeholder tools.
--
-- vehicles                      → manage_fleet (ТЗ §11.1: транспорт водителя/перевозчика)
-- warehouse_dispatch_thresholds → manage_warehouse_slots (ТЗ §5.2: порог отправки по направлению)
-- consolidated_customs_offers   → manage_customs_docs (ТЗ §10.2: конкурс таможенных представителей)

CREATE TABLE vehicles (
    id               uuid PRIMARY KEY,
    user_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    axles            int NOT NULL CHECK (axles >= 1 AND axles <= 12),
    capacity_kg      numeric NOT NULL CHECK (capacity_kg > 0),
    length_m         numeric NOT NULL CHECK (length_m > 0),
    width_m          numeric NOT NULL CHECK (width_m > 0),
    height_m         numeric NOT NULL CHECK (height_m > 0),
    -- Тип кузова свободным текстом (тентованный, открытая площадка, тралл…):
    -- набор типов — бизнес-данные, не схема; enum потребовал бы миграцию на
    -- каждый новый тип.
    body_type        text NOT NULL CHECK (length(trim(body_type)) > 0),
    -- Текущее местонахождение — свободный текст, обновляется в любое время.
    current_location text NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_vehicles_user_id ON vehicles (user_id);

-- Порог отправки привязан к направлению участника (participant_routes и есть
-- «направления» из ТЗ). accrued_m3 — сколько уже набрано; заполняется складом
-- самостоятельно (автоматический пересчёт из сделок — следующий этап).
CREATE TABLE warehouse_dispatch_thresholds (
    route_id     uuid PRIMARY KEY REFERENCES participant_routes (id) ON DELETE CASCADE,
    threshold_m3 numeric NOT NULL CHECK (threshold_m3 > 0),
    accrued_m3   numeric NOT NULL DEFAULT 0 CHECK (accrued_m3 >= 0),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- Предложения таможенных представителей на закрытую (matched) консолидацию.
-- Один представитель — одно предложение на консолидацию.
CREATE TABLE consolidated_customs_offers (
    id                      uuid PRIMARY KEY,
    consolidated_request_id uuid NOT NULL REFERENCES consolidated_requests (id) ON DELETE CASCADE,
    customs_rep_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    price                   numeric NOT NULL CHECK (price > 0),
    currency                text NOT NULL DEFAULT 'KZT',
    conditions              text NOT NULL DEFAULT '',
    status                  text NOT NULL DEFAULT 'submitted'
                            CHECK (status IN ('submitted', 'selected', 'rejected')),
    created_at              timestamptz NOT NULL DEFAULT now(),
    UNIQUE (consolidated_request_id, customs_rep_id)
);

CREATE INDEX idx_customs_offers_consolidated ON consolidated_customs_offers (consolidated_request_id);
