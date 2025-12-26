-- Migration: Add organization and user tracking to hosts table
-- This migration adds org_id (foreign key to organizations) and uploaded_by_user_id (foreign key to users) columns to hosts

-- Step 1: Add org_id column (nullable, for backward compatibility)
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS org_id UUID;

-- Step 2: Add uploaded_by_user_id column (nullable, for backward compatibility)
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS uploaded_by_user_id UUID;

-- Step 3: Add foreign key constraint for org_id
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_hosts_org_id'
        AND conrelid = 'hosts'::regclass
    ) THEN
        ALTER TABLE hosts
        ADD CONSTRAINT fk_hosts_org_id
        FOREIGN KEY (org_id)
        REFERENCES organizations(id)
        ON DELETE SET NULL;
    END IF;
END$$;

-- Step 4: Add foreign key constraint for uploaded_by_user_id
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_hosts_uploaded_by_user_id'
        AND conrelid = 'hosts'::regclass
    ) THEN
        ALTER TABLE hosts
        ADD CONSTRAINT fk_hosts_uploaded_by_user_id
        FOREIGN KEY (uploaded_by_user_id)
        REFERENCES users(id)
        ON DELETE SET NULL;
    END IF;
END$$;

-- Step 5: Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_hosts_org_id ON hosts(org_id);
CREATE INDEX IF NOT EXISTS idx_hosts_uploaded_by_user_id ON hosts(uploaded_by_user_id);

