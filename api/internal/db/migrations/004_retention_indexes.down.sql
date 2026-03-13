DROP INDEX IF EXISTS idx_containers_ports;
DROP INDEX IF EXISTS idx_containers_env_vars;
DROP INDEX IF EXISTS idx_containers_labels;

SELECT remove_retention_policy('container_events', if_exists => TRUE);
SELECT remove_compression_policy('container_events', if_exists => TRUE);

ALTER TABLE container_events SET (timescaledb.compress = false);
