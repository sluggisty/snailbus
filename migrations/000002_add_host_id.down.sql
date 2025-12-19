-- Rollback migration: Remove host_id and restore hostname as primary key

-- Step 1: Drop index on hostname
DROP INDEX IF EXISTS idx_hosts_hostname;

-- Step 2: Drop unique constraint on hostname
ALTER TABLE hosts DROP CONSTRAINT IF EXISTS hosts_hostname_unique;

-- Step 3: Drop primary key on host_id
ALTER TABLE hosts DROP CONSTRAINT IF EXISTS hosts_pkey;

-- Step 4: Restore primary key on hostname
ALTER TABLE hosts ADD PRIMARY KEY (hostname);

-- Step 5: Remove host_id column
ALTER TABLE hosts DROP COLUMN IF EXISTS host_id;

