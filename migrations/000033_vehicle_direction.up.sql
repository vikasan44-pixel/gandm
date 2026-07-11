-- Публичный поиск транспорта (запрос владельца, 2026-07-11): у машины
-- появляется опциональное объявленное направление «готов везти откуда → куда».
-- Хранится КООРДИНАТАМИ + подписью/страной (как груз и маршруты), чтобы
-- поиск шёл по радиусу haversine — единый способ на всей платформе, без
-- текстового матчинга (иначе «Алматы»/«Almaty» не сведутся).
ALTER TABLE vehicles
    ADD COLUMN ready_origin_lat        double precision,
    ADD COLUMN ready_origin_lng        double precision,
    ADD COLUMN ready_origin_label      text NOT NULL DEFAULT '',
    ADD COLUMN ready_origin_country    text NOT NULL DEFAULT '',
    ADD COLUMN ready_destination_lat   double precision,
    ADD COLUMN ready_destination_lng   double precision,
    ADD COLUMN ready_destination_label text NOT NULL DEFAULT '',
    ADD COLUMN ready_destination_country text NOT NULL DEFAULT '';

-- Индексы под широтный префильтр публичного поиска (как в миграции 000030).
CREATE INDEX idx_vehicles_ready_origin_lat ON vehicles (ready_origin_lat) WHERE ready_origin_lat IS NOT NULL;
