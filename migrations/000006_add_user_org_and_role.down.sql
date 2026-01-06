-- Rollback migration: Remove organization and role columns from users table

-- Step 1: Drop indexes
DROP INDEX IF EXISTS idx_users_role;
DROP INDEX IF EXISTS idx_users_org_id;

-- Step 2: Drop foreign key constraint
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_users_org_id'
        AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users DROP CONSTRAINT fk_users_org_id;
    END IF;
END$$;

-- Step 3: Drop columns
ALTER TABLE users DROP COLUMN IF EXISTS role;
ALTER TABLE users DROP COLUMN IF EXISTS org_id;

-- Step 4: Drop enum type (only if no other tables use it)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_type t
        JOIN pg_enum e ON t.oid = e.enumtypid
        WHERE t.typname = 'user_role'
        AND EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE udt_name = 'user_role'
            AND table_name != 'users'
        )
    ) THEN
        DROP TYPE IF EXISTS user_role;
    END IF;
END$$;


