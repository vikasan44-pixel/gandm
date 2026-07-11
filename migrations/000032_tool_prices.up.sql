-- Инструменты вместо роли (запрос владельца, 2026-07-11): у каждого
-- инструмента появляется цена (₸/мес; 0 = бесплатный) и человеческое
-- описание. Принцип монетизации: «давать объявление / предлагать» —
-- бесплатно, «смотреть чужие объявления» — платно.
ALTER TABLE tools ADD COLUMN price_kzt numeric NOT NULL DEFAULT 0 CHECK (price_kzt >= 0);

-- Человеческие описания + цены. admin-инструменты остаются служебными и не
-- попадают в самовыбор при регистрации (фильтр category <> 'admin').

-- Бесплатные — публикация/предложение своих услуг:
UPDATE tools SET price_kzt = 0,
  name = 'Подать заявку на груз',
  description = 'Публикуйте объявления о своём грузе — бесплатно. Исполнители сами пришлют предложения.'
  WHERE key = 'create_cargo_request';

UPDATE tools SET price_kzt = 0,
  name = 'Мой транспорт',
  description = 'Опубликуйте свой транспорт (автопарк) и участвуйте в конкурсах на перевозку — бесплатно.'
  WHERE key = 'manage_fleet';

UPDATE tools SET price_kzt = 0,
  name = 'Мой склад',
  description = 'Опубликуйте склад: направления, пороги отправки, заполняемость — бесплатно.'
  WHERE key = 'manage_warehouse_slots';

UPDATE tools SET price_kzt = 0,
  name = 'Заполняемость склада',
  description = 'Публикуйте загрузку вашего склада с фото — повышает доверие клиентов. Бесплатно.'
  WHERE key = 'submit_fill_report';

UPDATE tools SET price_kzt = 0,
  name = 'Услуги таможенного представителя',
  description = 'Предлагайте оформление таможенных документов и участвуйте в конкурсах — бесплатно.'
  WHERE key = 'manage_customs_docs';

UPDATE tools SET price_kzt = 0,
  name = 'Подавать предложения',
  description = 'Откликайтесь на грузы своими предложениями — бесплатно (нужен инструмент просмотра грузов, чтобы их видеть).'
  WHERE key = 'submit_offer';

-- Платные — просмотр чужих объявлений (искать себе работу):
UPDATE tools SET price_kzt = 9900,
  name = 'Смотреть грузы по направлению',
  description = 'Получайте и просматривайте подходящие грузы по вашим направлениям, чтобы предлагать перевозку. Подписка.'
  WHERE key = 'receive_cargo_by_route';

UPDATE tools SET price_kzt = 9900,
  name = 'Просмотр всех заявок на груз',
  description = 'Просматривайте опубликованные заявки на груз. Подписка.'
  WHERE key = 'view_cargo_requests';

-- Служебные (admin) — описания для админки, ценой не участвуют:
UPDATE tools SET description = 'Служебный: просмотр очереди верификации, одобрение и отклонение заявок.' WHERE key = 'verify_participants';
UPDATE tools SET description = 'Служебный: просмотр списка и карточек участников платформы.' WHERE key = 'view_users';
UPDATE tools SET description = 'Служебный: создание инструментов, сборка наборов, назначение участникам.' WHERE key = 'manage_tools';
UPDATE tools SET description = 'Создание суб-аккаунтов сотрудников внутри аккаунта компании.' WHERE key = 'manage_employees';
