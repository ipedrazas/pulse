CREATE TABLE agents (
    node_name     TEXT PRIMARY KEY,
    agent_version TEXT        NOT NULL DEFAULT '',
    first_seen    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
