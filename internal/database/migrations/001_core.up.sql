CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Agents registered with the server
CREATE TABLE agents (
    id              UUID PRIMARY KEY,
    secret_hash     TEXT NOT NULL,
    hostname        TEXT NOT NULL,
    os              TEXT,
    platform        TEXT,
    arch            TEXT,
    cpu_model       TEXT,
    cpu_cores       INTEGER,
    ram_total       BIGINT,
    registered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One-time registration tokens
CREATE TABLE registration_tokens (
    token       TEXT PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL,
    used        BOOLEAN NOT NULL DEFAULT FALSE,
    used_by     UUID REFERENCES agents(id)
);

-- Current-state tables (upserted, no history)
CREATE TABLE current_processes (
    agent_id    UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    pid         INTEGER NOT NULL,
    name        TEXT,
    cpu_percent DOUBLE PRECISION,
    mem_percent DOUBLE PRECISION,
    mem_rss     BIGINT,
    status      TEXT,
    threads     INTEGER,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, pid)
);

CREATE TABLE current_services (
    agent_id    UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    status      TEXT,
    sub_status  TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, name)
);

CREATE TABLE current_applications (
    agent_id    UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    version     TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_id, name)
);