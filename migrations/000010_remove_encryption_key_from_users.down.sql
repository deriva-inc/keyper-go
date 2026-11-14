-- SECTION: Restore encryption_key to users table
ALTER TABLE users
ADD COLUMN IF NOT EXISTS encryption_key TEXT;
-- !SECTION: Restore encryption_key to users table

