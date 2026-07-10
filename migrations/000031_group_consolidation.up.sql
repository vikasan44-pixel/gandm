-- Групповая консолидация (ТЗ §4.2 «два клиента И БОЛЕЕ»): вместо пар —
-- группы до лимита вместимости (7-8 клиентов в одной консолидации, лишь бы
-- влезали). Пара остаётся частным случаем группы из двух.

-- Участники предложения об объединении, каждый со своим ответом. Старые
-- парные колонки cargo_request_a/b становятся legacy (nullable), данные
-- переносятся в участников.
CREATE TABLE consolidation_suggestion_members (
    suggestion_id    uuid NOT NULL REFERENCES consolidation_suggestions (id) ON DELETE CASCADE,
    cargo_request_id uuid NOT NULL REFERENCES cargo_requests (id) ON DELETE CASCADE,
    client_id        uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    response         text NOT NULL DEFAULT 'pending' CHECK (response IN ('pending', 'agreed', 'declined')),
    PRIMARY KEY (suggestion_id, cargo_request_id)
);

CREATE INDEX idx_suggestion_members_cargo ON consolidation_suggestion_members (cargo_request_id);

ALTER TABLE consolidation_suggestions ALTER COLUMN cargo_request_a DROP NOT NULL;
ALTER TABLE consolidation_suggestions ALTER COLUMN cargo_request_b DROP NOT NULL;

-- Перенос существующих парных предложений в участников.
INSERT INTO consolidation_suggestion_members (suggestion_id, cargo_request_id, client_id, response)
SELECT s.id, s.cargo_request_a, c.client_id,
       CASE WHEN s.status IN ('a_agreed', 'both_agreed') THEN 'agreed' ELSE 'pending' END
FROM consolidation_suggestions s
JOIN cargo_requests c ON c.id = s.cargo_request_a
WHERE s.cargo_request_a IS NOT NULL
ON CONFLICT DO NOTHING;

INSERT INTO consolidation_suggestion_members (suggestion_id, cargo_request_id, client_id, response)
SELECT s.id, s.cargo_request_b, c.client_id,
       CASE WHEN s.status IN ('b_agreed', 'both_agreed') THEN 'agreed' ELSE 'pending' END
FROM consolidation_suggestions s
JOIN cargo_requests c ON c.id = s.cargo_request_b
WHERE s.cargo_request_b IS NOT NULL
ON CONFLICT DO NOTHING;

-- Принятия платного объединения: в группе принимает каждый участник
-- отдельно (инициатор считается принявшим с момента приглашения).
-- invited_client_id у consolidated_requests становится legacy.
CREATE TABLE consolidated_acceptances (
    consolidated_request_id uuid NOT NULL REFERENCES consolidated_requests (id) ON DELETE CASCADE,
    client_id               uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    accepted_at             timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (consolidated_request_id, client_id)
);

-- Перенос: в принятых парных объединениях приняли оба.
INSERT INTO consolidated_acceptances (consolidated_request_id, client_id)
SELECT cr.id, x.cid
FROM consolidated_requests cr
CROSS JOIN LATERAL (VALUES (cr.initiator_client_id), (cr.invited_client_id)) AS x(cid)
WHERE cr.invite_status = 'accepted' AND x.cid IS NOT NULL
ON CONFLICT DO NOTHING;
