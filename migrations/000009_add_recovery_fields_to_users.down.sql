-- SECTION: Revert recovery fields in users table
ALTER TABLE users
DROP COLUMN IF EXISTS recovery_salt;
-- !SECTION: Revert recovery fields in users table
