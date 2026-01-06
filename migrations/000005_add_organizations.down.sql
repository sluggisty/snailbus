-- Rollback migration: Remove organizations table

DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;
DROP INDEX IF EXISTS idx_organizations_name;
DROP TABLE IF EXISTS organizations;


