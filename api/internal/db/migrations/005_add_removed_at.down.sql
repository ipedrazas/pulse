DROP INDEX IF EXISTS idx_containers_active;
ALTER TABLE containers DROP COLUMN IF EXISTS removed_at;
