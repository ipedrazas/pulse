ALTER TABLE containers ADD COLUMN compose_project TEXT NOT NULL DEFAULT '';
ALTER TABLE containers ADD COLUMN compose_dir TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_containers_compose_project ON containers (compose_project) WHERE compose_project != '';
