DROP INDEX IF EXISTS idx_containers_compose_project;
ALTER TABLE containers DROP COLUMN compose_dir;
ALTER TABLE containers DROP COLUMN compose_project;
