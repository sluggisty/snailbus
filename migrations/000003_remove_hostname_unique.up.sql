-- Migration: Remove unique constraint on hostname
-- This allows multiple hosts to have the same hostname, as they are identified by host_id

-- Drop the unique constraint on hostname
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

-- Note: The index on hostname (idx_hosts_hostname) is kept for faster lookups
-- even though hostname is no longer unique

