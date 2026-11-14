-- SECTION: Remove encryption_key from users table
ALTER TABLE users
DROP COLUMN IF EXISTS encryption_key;
-- !SECTION: Remove encryption_key from users table

