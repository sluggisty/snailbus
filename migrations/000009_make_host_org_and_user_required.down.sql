-- Rollback migration: Make org_id and uploaded_by_user_id nullable in hosts table

-- Step 1: Make uploaded_by_user_id nullable
ALTER TABLE hosts ALTER COLUMN uploaded_by_user_id DROP NOT NULL;

-- Step 2: Make org_id nullable
ALTER TABLE hosts ALTER COLUMN org_id DROP NOT NULL;

-- Step 3: Update foreign key constraint for uploaded_by_user_id back to SET NULL
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_hosts_uploaded_by_user_id'
        AND conrelid = 'hosts'::regclass
    ) THEN
        -- Drop existing constraint
        ALTER TABLE hosts DROP CONSTRAINT fk_hosts_uploaded_by_user_id;
        
        -- Recreate with SET NULL
        ALTER TABLE hosts
        ADD CONSTRAINT fk_hosts_uploaded_by_user_id
        FOREIGN KEY (uploaded_by_user_id)
        REFERENCES users(id)
        ON DELETE SET NULL;
    END IF;
END$$;

-- Step 4: Update foreign key constraint for org_id back to SET NULL
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_hosts_org_id'
        AND conrelid = 'hosts'::regclass
    ) THEN
        -- Drop existing constraint
        ALTER TABLE hosts DROP CONSTRAINT fk_hosts_org_id;
        
        -- Recreate with SET NULL
        ALTER TABLE hosts
        ADD CONSTRAINT fk_hosts_org_id
        FOREIGN KEY (org_id)
        REFERENCES organizations(id)
        ON DELETE SET NULL;
    END IF;
END$$;


