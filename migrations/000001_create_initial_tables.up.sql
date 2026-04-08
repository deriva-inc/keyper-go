
-- Extension for UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- SECTION: `users` table - Stores user account and authentication data.
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT NOT NULL UNIQUE,
    auth_hash TEXT NOT NULL,
    salt TEXT NOT NULL,
    recovery_hash TEXT,
    display_name TEXT,
    avatar_url TEXT,

    -- Timestamps for creation and last update.
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_users_lower_email ON users(LOWER(email));

-- Create a trigger function
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Set the trigger function on the table using the function created above.
CREATE TRIGGER set_users_timestamp
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();
-- !SECTION: `users` table

-- SECTION: `profiles` table - Represents a user's organizational context (e.g., 'Work', 'Personal').
CREATE TABLE profiles (
	id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- The name of the profile (e.g., 'Work', 'Personal').
    name TEXT NOT NULL,

    -- An optional text field for an icon url/name.
    icon TEXT,

    -- Timestamps for creation and last update.
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT unique_user_profile_name UNIQUE(user_id, name)
);

-- Create an index on profiles
CREATE INDEX idx_profiles_user_id ON profiles(user_id);

-- Set the same trigger on the `profiles` table
CREATE TRIGGER set_profiles_timestamp
BEFORE UPDATE ON profiles
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();
-- !SECTION: `profiles` table

-- SECTION: `groups` table - Represents a user-defined folder to group related vault entries.
CREATE TABLE groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,

    -- The name of the group (e.g., 'Amazon Ecosystem', 'Zerodha Accounts').
    name TEXT NOT NULL,

    -- An optional text field for an icon url/name.
    icon TEXT,

    -- Timestamps for creation and last update.
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- A user should not have two groups with the same name within the same profile.
    CONSTRAINT unique_profile_group_name UNIQUE(profile_id, name)
);

-- Index to quickly find all groups belonging to a profile.
CREATE INDEX idx_groups_profile_id ON groups(profile_id);

-- Apply the trigger to automatically update the `updated_at` timestamp.
CREATE TRIGGER set_groups_timestamp
BEFORE UPDATE ON groups
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();
-- !SECTION: `groups` table

-- SECTION: `vault_entries` table - Stores individual encrypted records.
-- First, define a custom ENUM type for the different kinds of entries.
-- This is more efficient and safer than using a plain TEXT field.
CREATE TYPE entry_type AS ENUM (
    'login',            -- For websites and apps (e.g., Swiggy, Zerodha)
    'credit_card',      -- Standard credit cards
    'debit_card',       -- Standard debit cards
    'bank_account',     -- For storing account/IFSC numbers
    'upi_id',           -- CRITICAL for India: To store UPI Virtual Payment Addresses (VPAs)
    'identity_card',    -- For Aadhaar, PAN card, Voter ID, Passport
    'secure_note'       -- A general-purpose encrypted note for anything else
);

CREATE TABLE vault_entries (
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

-- Index to quickly find all entries in a profile.
CREATE INDEX idx_vault_entries_profile_id ON vault_entries(profile_id);

-- Index to quickly find all entries in a specific group.
CREATE INDEX idx_vault_entries_group_id ON vault_entries(group_id);

-- Apply the trigger to automatically update the `updated_at` timestamp.
CREATE TRIGGER set_vault_entries_timestamp
BEFORE UPDATE ON vault_entries
FOR EACH ROW
EXECUTE FUNCTION trigger_set_timestamp();
-- !SECTION: `vault_entries` table
