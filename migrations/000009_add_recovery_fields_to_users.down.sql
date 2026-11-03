-- SECTION: Revert recovery fields in users table
ALTER TABLE users
DROP COLUMN IF EXISTS recovery_salt;

ALTER TABLE users
DROP COLUMN IF EXISTS recovery_master_key;
-- !SECTION: Revert recovery fields in users table
