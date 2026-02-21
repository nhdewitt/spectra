CREATE TABLE current_metrics (
    agent_id            UUID PRIMARY KEY REFERENCES agents(id) ON DELETE CASCADE,
    cpu_usage           DOUBLE PRECISION,
    load_normalized     DOUBLE PRECISION,
    ram_percent         DOUBLE PRECISION,
    swap_percent        DOUBLE PRECISION,
    disk_max_percent    DOUBLE PRECISION,
    net_rx_bytes        BIGINT,
    net_tx_bytes        BIGINT,
    max_temp            DOUBLE PRECISION,
    uptime              BIGINT,
    process_count       INTEGER,
    reboot_required     BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);