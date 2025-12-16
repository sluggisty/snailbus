-- Create hosts table to store system information from snail-core
CREATE TABLE IF NOT EXISTS hosts (
    hostname TEXT PRIMARY KEY,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    collection_id TEXT,
    timestamp TEXT,
    snail_version TEXT,
    data JSONB NOT NULL,
    errors TEXT[]
);

-- Create index on received_at for efficient querying
CREATE INDEX IF NOT EXISTS idx_hosts_received_at ON hosts(received_at DESC);

-- Create index on collection_id for lookups
CREATE INDEX IF NOT EXISTS idx_hosts_collection_id ON hosts(collection_id);

