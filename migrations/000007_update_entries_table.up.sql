
-- Extension for UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- SECTION: `users` table - Stores user account and authentication data.
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT NOT NULL UNIQUE,
    auth_key TEXT NOT NULL,
    auth_salt TEXT NOT NULL,
    encryption_key TEXT NOT NULL,
    encryption_salt TEXT NOT NULL,
    recovery_key TEXT,
    name TEXT,
    avatar_url TEXT,

    -- Timestamps for creation and last update.
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_lower_email ON users(LOWER(email));

-- Create a trigger function
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- set the trigger function on the table using function created above
-- Drop and recreate the trigger to ensure idempotency
DROP TRIGGER IF EXISTS set_users_timestamp ON users;
CREATE TRIGGER set_users_timestamp
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();
-- !SECTION: `users` table

-- SECTION: `profiles` table - Represents a user's organizational context (e.g., 'Work', 'Personal').
CREATE TABLE IF NOT EXISTS profiles (
	id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- The name of the profile (e.g., 'Work', 'Personal').
    name TEXT NOT NULL,

    -- An optional text field for an icon url/name.
    icon TEXT,
    description TEXT,

    -- Timestamps for creation and last update.
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT unique_user_profile_name UNIQUE(user_id, name)
);

ALTER TABLE profiles
ADD COLUMN IF NOT EXISTS is_archived BOOLEAN NOT NULL DEFAULT FALSE;

-- Create an index on profiles
CREATE INDEX IF NOT EXISTS idx_profiles_user_id ON profiles(user_id);

-- Set the same trigger on the `profiles` table
-- Drop and recreate the trigger to ensure idempotency
DROP TRIGGER IF EXISTS set_profiles_timestamp ON profiles;
CREATE TRIGGER set_profiles_timestamp
BEFORE UPDATE ON profiles
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();
-- !SECTION: `profiles` table

-- SECTION: `groups` table - Represents a user-defined folder to group related vault entries.
-- Create group_type enum if it does not exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'group_type') THEN
        CREATE TYPE group_type AS ENUM ('provider', 'category');
    END IF;
END$$;

--
-- `groups` table: Represents a user-defined folder to group related vault entries.
-- A group must belong to a profile.
--
CREATE TABLE IF NOT EXISTS groups (
    -- Primary Key for the groups table.
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Foreign Key linking this group to a profile. This is NOT NULL.
    -- If a profile is deleted, all its groups are also deleted.
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,

    -- The name of the group (e.g., 'Amazon Ecosystem', 'Social Media').
    name TEXT NOT NULL,
    description TEXT,

    type group_type NOT NULL,

    -- An optional text field for an icon name.
    icon TEXT,

    -- Timestamps for creation and last update.
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- A user should not have two groups with the same name within the same profile.
    CONSTRAINT unique_profile_group_name UNIQUE(profile_id, name)
);

-- Add 'type' column of enum type 'group_type'
ALTER TABLE groups
ADD COLUMN IF NOT EXISTS type group_type NOT NULL DEFAULT 'provider';

-- Add 'description' column of type TEXT
ALTER TABLE groups
ADD COLUMN IF NOT EXISTS description TEXT;

-- Index to quickly find all groups belonging to a profile.
CREATE INDEX IF NOT EXISTS idx_groups_profile_id ON groups(profile_id);

-- Apply the trigger to automatically update the `updated_at` timestamp.
-- Drop and recreate the trigger to ensure idempotency
DROP TRIGGER IF EXISTS set_groups_timestamp ON groups;
CREATE TRIGGER set_groups_timestamp
BEFORE UPDATE ON groups
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();
-- !SECTION: `groups` table

-- SECTION: `vault_entries` table - Stores individual encrypted records.
-- First, define a custom ENUM type for the different kinds of entries.
-- This is more efficient and safer than using a plain TEXT field.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'entry_type') THEN
        CREATE TYPE entry_type AS ENUM (
            'login',
            'credit_card',
            'debit_card',
            'bank_account',
            'upi_id',
            'identity_card',
            'secure_note'
        );
    END IF;
END$$;

CREATE TABLE IF NOT EXISTS vault_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    
    -- Foreign Key to the groups table. This is NULLABLE.
    -- If NULL, it's a standalone entry. If it has a value, it's part of a group.
    group_id UUID REFERENCES groups(id) ON DELETE SET NULL, -- If a group is deleted, entries become standalone.
    
    -- The type of entry, using our custom ENUM type.
    type entry_type NOT NULL,
    
    -- The user-friendly name for the entry (e.g., "GitHub Account").
    name TEXT NOT NULL,
    
    -- This single column stores the fully encrypted data as a binary blob.
    -- Your client application is responsible for encrypting/decrypting this.
    encrypted_blob BYTEA NOT NULL,
    
    -- A flexible field for any extra key-value pairs.
    custom_fields JSONB,
    is_favorite BOOLEAN NOT NULL DEFAULT FALSE,

    -- Timestamps for creation and last update.
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE vault_entries
ADD COLUMN IF NOT EXISTS icon TEXT;

ALTER TABLE vault_entries
ADD COLUMN IF NOT EXISTS description TEXT;

ALTER TABLE vault_entries
ADD COLUMN IF NOT EXISTS website_url TEXT NOT NULL DEFAULT '';

ALTER TABLE vault_entries
ADD COLUMN IF NOT EXISTS user_id TEXT;

ALTER TABLE vault_entries
ADD COLUMN IF NOT EXISTS user_name TEXT;

ALTER TABLE vault_entries
ADD COLUMN IF NOT EXISTS card_number TEXT;

ALTER TABLE vault_entries
ADD COLUMN IF NOT EXISTS expiration_date TEXT;

ALTER TABLE vault_entries
ADD COLUMN IF NOT EXISTS security_code TEXT;

ALTER TABLE vault_entries
ADD COLUMN IF NOT EXISTS email TEXT NOT NULL DEFAULT '';

ALTER TABLE vault_entries
ADD COLUMN IF NOT EXISTS is_archived BOOLEAN NOT NULL DEFAULT FALSE;

-- Index to quickly find all entries in a profile.
CREATE INDEX IF NOT EXISTS idx_vault_entries_profile_id ON vault_entries(profile_id);

-- Index to quickly find all entries in a specific group.
CREATE INDEX IF NOT EXISTS idx_vault_entries_group_id ON vault_entries(group_id);

-- Apply the trigger to automatically update the `updated_at` timestamp.
-- Drop and recreate the trigger to ensure idempotency
DROP TRIGGER IF EXISTS set_vault_entries_timestamp ON vault_entries;
CREATE TRIGGER set_vault_entries_timestamp
BEFORE UPDATE ON vault_entries
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();
-- !SECTION: `vault_entries` table
