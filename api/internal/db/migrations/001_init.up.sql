-- Containers metadata table
CREATE TABLE IF NOT EXISTS containers (
    container_id TEXT PRIMARY KEY,
    node_name    TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    image_tag    TEXT        NOT NULL,
    env_vars     JSONB       NOT NULL DEFAULT '{}',
    mounts       JSONB       NOT NULL DEFAULT '[]',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_containers_node_name ON containers (node_name);

-- Heartbeats time-series table
CREATE TABLE IF NOT EXISTS container_heartbeats (
    time           TIMESTAMPTZ NOT NULL,
    container_id   TEXT        NOT NULL REFERENCES containers (container_id) ON DELETE CASCADE,
    status         TEXT        NOT NULL,
    uptime_seconds BIGINT      NOT NULL
);

SELECT create_hypertable('container_heartbeats', 'time');

CREATE INDEX idx_heartbeats_container_id ON container_heartbeats (container_id, time DESC);

-- Compress chunks older than 7 days
ALTER TABLE container_heartbeats SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'container_id',
    timescaledb.compress_orderby = 'time DESC'
);

SELECT add_compression_policy('container_heartbeats', INTERVAL '7 days');

-- Drop chunks older than 30 days
SELECT add_retention_policy('container_heartbeats', INTERVAL '30 days');
