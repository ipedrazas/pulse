-- Soft-delete support: mark containers that no longer exist on their node.
ALTER TABLE containers ADD COLUMN removed_at TIMESTAMPTZ;

-- Partial index: most queries only care about active (non-removed) containers.
CREATE INDEX idx_containers_active ON containers (node_name, name) WHERE removed_at IS NULL;
