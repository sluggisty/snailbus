-- Migration: Make org_id and role required in users table
-- This migration makes org_id and role NOT NULL (requires existing data to have values)

-- Step 1: Update foreign key constraint to use RESTRICT instead of SET NULL
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_users_org_id'
        AND conrelid = 'users'::regclass
    ) THEN
        -- Drop existing constraint
        ALTER TABLE users DROP CONSTRAINT fk_users_org_id;
        
        -- Recreate with RESTRICT
        ALTER TABLE users
        ADD CONSTRAINT fk_users_org_id
        FOREIGN KEY (org_id)
        REFERENCES organizations(id)
        ON DELETE RESTRICT;
    END IF;
END$$;

-- Step 2: Make org_id NOT NULL
-- Note: This will fail if there are existing users with NULL org_id
-- Ensure all users have an org_id before running this migration
ALTER TABLE users ALTER COLUMN org_id SET NOT NULL;

-- Step 3: Make role NOT NULL
-- Note: This will fail if there are existing users with NULL role
-- Ensure all users have a role before running this migration
ALTER TABLE users ALTER COLUMN role SET NOT NULL;


