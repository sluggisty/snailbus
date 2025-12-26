-- Migration: Add organization and role support to users table
-- This migration adds org_id (foreign key to organizations) and role (enum) columns to users

-- Step 1: Create user_role enum type
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
        CREATE TYPE user_role AS ENUM ('admin', 'editor', 'viewer');
    END IF;
END$$;

-- Step 2: Add org_id column (nullable, for backward compatibility)
ALTER TABLE users ADD COLUMN IF NOT EXISTS org_id UUID;

-- Step 3: Add role column (nullable, for backward compatibility)
ALTER TABLE users ADD COLUMN IF NOT EXISTS role user_role;

-- Step 4: Add foreign key constraint for org_id
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_users_org_id'
        AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users
        ADD CONSTRAINT fk_users_org_id
        FOREIGN KEY (org_id)
        REFERENCES organizations(id)
        ON DELETE SET NULL;
    END IF;
END$$;

-- Step 5: Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_org_id ON users(org_id);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

