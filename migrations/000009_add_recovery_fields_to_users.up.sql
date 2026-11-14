-- SECTION: Update recovery fields in users table
ALTER TABLE users
ADD COLUMN IF NOT EXISTS recovery_salt TEXT;
-- !SECTION: Update recovery fields in users table
