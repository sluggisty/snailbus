-- Rollback migration: Restore unique constraint on hostname

-- Add back the unique constraint on hostname
ALTER TABLE hosts ADD CONSTRAINT hosts_hostname_unique UNIQUE (hostname);


