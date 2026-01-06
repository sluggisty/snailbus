-- Rollback migration: Remove organization and user tracking columns from hosts table

-- Step 1: Drop indexes
DROP INDEX IF EXISTS idx_hosts_uploaded_by_user_id;
DROP INDEX IF EXISTS idx_hosts_org_id;

-- Step 2: Drop foreign key constraint for uploaded_by_user_id
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_hosts_uploaded_by_user_id'
        AND conrelid = 'hosts'::regclass
    ) THEN
        ALTER TABLE hosts DROP CONSTRAINT fk_hosts_uploaded_by_user_id;
    END IF;
END$$;

-- Step 3: Drop foreign key constraint for org_id
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_hosts_org_id'
        AND conrelid = 'hosts'::regclass
    ) THEN
        ALTER TABLE hosts DROP CONSTRAINT fk_hosts_org_id;
    END IF;
END$$;

-- Step 4: Drop columns
ALTER TABLE hosts DROP COLUMN IF EXISTS uploaded_by_user_id;
ALTER TABLE hosts DROP COLUMN IF EXISTS org_id;


