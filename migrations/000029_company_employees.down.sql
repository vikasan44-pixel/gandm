DELETE FROM tools WHERE key = 'manage_employees';
DROP INDEX IF EXISTS idx_users_parent_company;
ALTER TABLE users DROP COLUMN parent_company_id;
