-- Migration: Make org_id and uploaded_by_user_id required in hosts table
-- This migration makes org_id and uploaded_by_user_id NOT NULL (requires existing data to have values)

-- Step 1: Update foreign key constraint for org_id to use RESTRICT instead of SET NULL
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_hosts_org_id'
        AND conrelid = 'hosts'::regclass
    ) THEN
        -- Drop existing constraint
        ALTER TABLE hosts DROP CONSTRAINT fk_hosts_org_id;
        
        -- Recreate with RESTRICT
        ALTER TABLE hosts
        ADD CONSTRAINT fk_hosts_org_id
        FOREIGN KEY (org_id)
        REFERENCES organizations(id)
        ON DELETE RESTRICT;
    END IF;
END$$;

-- Step 2: Update foreign key constraint for uploaded_by_user_id to use RESTRICT instead of SET NULL
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_hosts_uploaded_by_user_id'
        AND conrelid = 'hosts'::regclass
    ) THEN
        -- Drop existing constraint
        ALTER TABLE hosts DROP CONSTRAINT fk_hosts_uploaded_by_user_id;
        
        -- Recreate with RESTRICT
        ALTER TABLE hosts
        ADD CONSTRAINT fk_hosts_uploaded_by_user_id
        FOREIGN KEY (uploaded_by_user_id)
        REFERENCES users(id)
        ON DELETE RESTRICT;
    END IF;
END$$;

-- Step 3: Make org_id NOT NULL
-- Note: This will fail if there are existing hosts with NULL org_id
-- Ensure all hosts have an org_id before running this migration
ALTER TABLE hosts ALTER COLUMN org_id SET NOT NULL;

-- Step 4: Make uploaded_by_user_id NOT NULL
-- Note: This will fail if there are existing hosts with NULL uploaded_by_user_id
-- Ensure all hosts have an uploaded_by_user_id before running this migration
ALTER TABLE hosts ALTER COLUMN uploaded_by_user_id SET NOT NULL;

