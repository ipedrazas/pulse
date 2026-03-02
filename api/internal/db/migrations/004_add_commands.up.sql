CREATE TABLE commands (
    command_id  TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    node_name   TEXT        NOT NULL,
    action      TEXT        NOT NULL,
    target      TEXT        NOT NULL DEFAULT '',
    params      JSONB       NOT NULL DEFAULT '{}',
    status      TEXT        NOT NULL DEFAULT 'pending',
    output      TEXT        NOT NULL DEFAULT '',
    duration_ms BIGINT      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_commands_node_status ON commands (node_name, status);
