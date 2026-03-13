-- Enable compression on container_events hypertable
ALTER TABLE container_events SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'container_id, agent_name',
    timescaledb.compress_orderby = 'time DESC'
);

-- Compress chunks older than 7 days
SELECT add_compression_policy('container_events', INTERVAL '7 days', if_not_exists => TRUE);

-- Drop data older than 30 days
SELECT add_retention_policy('container_events', INTERVAL '30 days', if_not_exists => TRUE);

-- GIN indexes for JSONB queries on containers
CREATE INDEX IF NOT EXISTS idx_containers_labels ON containers USING GIN (labels);
CREATE INDEX IF NOT EXISTS idx_containers_env_vars ON containers USING GIN (env_vars);
CREATE INDEX IF NOT EXISTS idx_containers_ports ON containers USING GIN (ports);
