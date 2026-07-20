UPDATE tools
SET is_active = true
WHERE key = 'submit_fill_report';

UPDATE tools
SET description = 'Опубликуйте склад: направления, пороги отправки, заполняемость — бесплатно.'
WHERE key = 'manage_warehouse_slots';
