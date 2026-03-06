-- Agents table: tracks connected compute nodes
CREATE TABLE IF NOT EXISTS agents (
    name        TEXT PRIMARY KEY,
    status      TEXT NOT NULL DEFAULT 'offline',
    version     TEXT NOT NULL DEFAULT '',
    last_seen   TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Containers table: current state of containers across all agents
CREATE TABLE IF NOT EXISTS containers (
    container_id    TEXT NOT NULL,
    agent_name      TEXT NOT NULL REFERENCES agents(name) ON DELETE CASCADE,
    name            TEXT NOT NULL DEFAULT '',
    image           TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT '',
    env_vars        JSONB NOT NULL DEFAULT '{}',
    mounts          JSONB NOT NULL DEFAULT '[]',
    labels          JSONB NOT NULL DEFAULT '{}',
    ports           JSONB NOT NULL DEFAULT '[]',
    compose_project TEXT NOT NULL DEFAULT '',
    command         TEXT NOT NULL DEFAULT '',
    uptime_seconds  BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    removed_at      TIMESTAMPTZ,
    PRIMARY KEY (container_id, agent_name)
);

CREATE INDEX idx_containers_agent ON containers(agent_name);
CREATE INDEX idx_containers_status ON containers(status);

-- Container events hypertable: time-series data
CREATE TABLE IF NOT EXISTS container_events (
    time            TIMESTAMPTZ NOT NULL,
    container_id    TEXT NOT NULL,
    agent_name      TEXT NOT NULL,
    status          TEXT NOT NULL,
    uptime_seconds  BIGINT NOT NULL DEFAULT 0
);

SELECT create_hypertable('container_events', 'time', if_not_exists => TRUE);

CREATE INDEX idx_events_container ON container_events(container_id, time DESC);

-- Commands table: queued commands for agents
CREATE TABLE IF NOT EXISTS commands (
    id              TEXT PRIMARY KEY,
    agent_name      TEXT NOT NULL REFERENCES agents(name) ON DELETE CASCADE,
    type            TEXT NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'pending',
    result          TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX idx_commands_agent_status ON commands(agent_name, status);
