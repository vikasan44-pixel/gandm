-- Суб-аккаунты сотрудников (ТЗ §13.1, вариант А): проверенная компания
-- создаёт логины для сотрудников внутри своего аккаунта. Сотрудник — обычный
-- участник со ссылкой на компанию; инструменты наследуются от компании
-- (см. ToolRepository.UserHasTool). Повторная верификация сотруднику не
-- нужна — компания уже проверена.
ALTER TABLE users ADD COLUMN parent_company_id uuid REFERENCES users (id) ON DELETE CASCADE;
CREATE INDEX idx_users_parent_company ON users (parent_company_id) WHERE parent_company_id IS NOT NULL;

-- Инструмент, дающий право создавать сотрудников (выдаётся админом
-- проверенным компаниям — в первую очередь таможенным представителям).
INSERT INTO tools (id, key, name, description, category, is_active)
VALUES (gen_random_uuid(), 'manage_employees', 'Сотрудники компании',
        'Создание суб-аккаунтов сотрудников внутри аккаунта компании (ТЗ §13.1)', 'admin', true)
ON CONFLICT (key) DO NOTHING;
