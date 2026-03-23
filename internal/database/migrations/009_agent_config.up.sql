-- Per-agent configuration stored as key-value pairs with JSONB values.
-- Used for display filtering (ignored filesystems/interfaces), labels, and future settings.

CREATE TABLE agent_config (
    agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    config_key      TEXT NOT NULL,
    config_value    JSONB NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, config_key)
);