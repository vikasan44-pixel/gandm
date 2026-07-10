-- Rate limiting логина через БД: in-memory лимитер не переживает
-- горизонтальное масштабирование — несколько инстансов за балансировщиком
-- делили бы счётчики. Таблица общая для всех инстансов.
CREATE TABLE login_attempts (
    ip           text NOT NULL,
    attempted_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_login_attempts_ip_time ON login_attempts (ip, attempted_at);

-- Гео-запросы матчинга: haversine_km(...) в WHERE не индексируется —
-- полный перебор строк. Префильтр по широте (sargable BETWEEN) + btree
-- индексы срезают кандидатов до узкой широтной полосы, точную проверку
-- по-прежнему делает haversine. Полное решение на больших объёмах —
-- PostGIS (geography + ST_DWithin), см. README.
CREATE INDEX idx_participant_routes_origin_lat ON participant_routes (origin_lat);
CREATE INDEX idx_participant_routes_destination_lat ON participant_routes (destination_lat);
CREATE INDEX idx_cargo_requests_status_origin_lat ON cargo_requests (status, origin_lat);
CREATE INDEX idx_consolidated_requests_status_origin_lat ON consolidated_requests (status, origin_lat);
