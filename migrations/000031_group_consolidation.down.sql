DROP TABLE IF EXISTS consolidated_acceptances;
DROP TABLE IF EXISTS consolidation_suggestion_members;
-- Парные колонки обратно NOT NULL не делаем: в них могли остаться NULL от
-- групповых предложений; down-миграция сознательно неполная.
