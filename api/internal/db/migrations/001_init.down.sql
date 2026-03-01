SELECT remove_retention_policy('container_heartbeats', if_exists => true);
SELECT remove_compression_policy('container_heartbeats', if_exists => true);
DROP TABLE IF EXISTS container_heartbeats;
DROP TABLE IF EXISTS containers;
