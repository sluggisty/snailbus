-- Rollback migration: Make org_id and role nullable in users table

-- Step 1: Make role nullable
ALTER TABLE users ALTER COLUMN role DROP NOT NULL;

-- Step 2: Make org_id nullable
ALTER TABLE users ALTER COLUMN org_id DROP NOT NULL;

-- Step 3: Update foreign key constraint back to SET NULL
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_users_org_id'
        AND conrelid = 'users'::regclass
    ) THEN
        -- Drop existing constraint
        ALTER TABLE users DROP CONSTRAINT fk_users_org_id;
        
        -- Recreate with SET NULL
        ALTER TABLE users
        ADD CONSTRAINT fk_users_org_id
        FOREIGN KEY (org_id)
        REFERENCES organizations(id)
        ON DELETE SET NULL;
    END IF;
END$$;

