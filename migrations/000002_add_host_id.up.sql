-- Migration: Add host_id support and change primary key from hostname to host_id
-- This migration adds host_id column and migrates existing data

-- Step 1: Add host_id column (nullable initially for migration)
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS host_id UUID;

-- Step 2: Generate UUIDs for existing hosts that don't have host_id
-- This ensures backward compatibility during migration
-- Note: For existing hosts without host_id, we generate a UUID based on hostname
-- This is a one-time migration - new hosts from snail-core will have host_id
UPDATE hosts 
SET host_id = gen_random_uuid() 
WHERE host_id IS NULL;

-- Step 3: Make host_id NOT NULL now that all rows have values
ALTER TABLE hosts ALTER COLUMN host_id SET NOT NULL;

-- Step 4: Drop the old primary key constraint (if it exists)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'hosts_pkey' 
        AND conrelid = 'hosts'::regclass
    ) THEN
        ALTER TABLE hosts DROP CONSTRAINT hosts_pkey;
    END IF;
END $$;

-- Step 5: Add new primary key on host_id
ALTER TABLE hosts ADD PRIMARY KEY (host_id);

-- Step 6: Add unique constraint on hostname (hostname can still change, but host_id is stable)
-- Drop existing unique constraint if it exists
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'hosts_hostname_unique' 
        AND conrelid = 'hosts'::regclass
    ) THEN
        ALTER TABLE hosts DROP CONSTRAINT hosts_hostname_unique;
    END IF;
END $$;
ALTER TABLE hosts ADD CONSTRAINT hosts_hostname_unique UNIQUE (hostname);

-- Step 7: Add index on hostname for faster lookups
CREATE INDEX IF NOT EXISTS idx_hosts_hostname ON hosts(hostname);

