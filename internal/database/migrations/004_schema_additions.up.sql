ALTER TABLE metrics_pi ADD COLUMN soft_temp_limit           BOOLEAN;
ALTER TABLE metrics_pi ADD COLUMN undervoltage_occurred     BOOLEAN;
ALTER TABLE metrics_pi ADD COLUMN freq_cap_occurred         BOOLEAN;
ALTER TABLE metrics_pi ADD COLUMN throttled_occurred        BOOLEAN;
ALTER TABLE metrics_pi ADD COLUMN soft_temp_limit_occurred  BOOLEAN;

ALTER TABLE metrics_wifi ADD COLUMN link_quality INTEGER;

CREATE TABLE IF NOT EXISTS current_updates (
    agent_id            UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    pending_count       INTEGER NOT NULL DEFAULT 0,
    security_count      INTEGER NOT NULL DEFAULT 0,
    reboot_required     BOOLEAN NOT NULL DEFAULT FALSE,
    package_manager     TEXT,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id)
);