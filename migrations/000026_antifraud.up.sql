-- Антинакрутка (ТЗ §6): «Избранное + Документы» — честный канал повторной
-- работы с тем же партнёром + документальное подтверждение повторных
-- сделок. Подозрительные паттерны считаются запросом по живым данным, без
-- отдельной таблицы.

CREATE TABLE favorites (
    client_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    participant_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (client_id, participant_id)
);

-- Документ, подтверждающий сделку (договор и т.п.). deal_id — id
-- cargo_request ИЛИ consolidated_request, как в ratings: FK невозможен,
-- принадлежность проверяет сервис.
CREATE TABLE deal_documents (
    id            uuid PRIMARY KEY,
    deal_id       uuid NOT NULL,
    uploader_id   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    file_url      text NOT NULL,
    original_name text NOT NULL,
    uploaded_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_deal_documents_deal ON deal_documents (deal_id);
