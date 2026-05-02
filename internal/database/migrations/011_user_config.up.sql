CREATE TABLE user_config (
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    config_key      TEXT NOT NULL,
    config_value    JSONB NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, config_key)
);