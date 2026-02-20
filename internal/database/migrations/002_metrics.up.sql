-- CPU metrics
CREATE TABLE metrics_cpu (
    time        TIMESTAMPTZ NOT NULL,
    agent_id    UUID NOT NULL,
    usage       DOUBLE PRECISION,
    core_usages DOUBLE PRECISION[],
    load_1m     DOUBLE PRECISION,
    load_5m     DOUBLE PRECISION,
    load_15m    DOUBLE PRECISION,
    iowait      DOUBLE PRECISION
);
SELECT create_hypertable('metrics_cpu', 'time');

-- Memory metrics
CREATE TABLE metrics_memory (
    time            TIMESTAMPTZ NOT NULL,
    agent_id        UUID NOT NULL,
    ram_total       BIGINT,
    ram_used        BIGINT,
    ram_available   BIGINT,
    ram_percent     DOUBLE PRECISION,
    swap_total      BIGINT,
    swap_used       BIGINT,
    swap_percent    DOUBLE PRECISION
);
SELECT create_hypertable('metrics_memory', 'time');

-- Disk uage metrics
CREATE TABLE metrics_disk (
    time            TIMESTAMPTZ NOT NULL,
    agent_id        UUID NOT NULL,
    device          TEXT,
    mountpoint      TEXT,
    filesystem      TEXT,
    disk_type       TEXT,
    total_bytes     BIGINT,
    used_bytes      BIGINT,
    free_bytes      BIGINT,
    used_percent    DOUBLE PRECISION,
    inodes_total    BIGINT,
    inodes_used     BIGINT,
    inodes_percent  DOUBLE PRECISION
);
SELECT create_hypertable('metrics_disk', 'time');

-- Disk I/O metrics
CREATE TABLE metrics_disk_io (
    time            TIMESTAMPTZ NOT NULL,
    agent_id        UUID NOT NULL,
    device          TEXT,
    read_bytes      BIGINT,
    write_bytes     BIGINT,
    read_ops        BIGINT,
    write_ops       BIGINT,
    read_latency    BIGINT,
    write_latency   BIGINT,
    io_in_progress  BIGINT
);
SELECT create_hypertable('metrics_disk_io', 'time');

-- Network metrics
CREATE TABLE metrics_network (
    time        TIMESTAMPTZ NOT NULL,
    agent_id    UUID NOT NULL,
    interface   TEXT,
    mac         TEXT,
    mtu         INTEGER,
    speed       BIGINT,
    rx_bytes    BIGINT,
    rx_packets  BIGINT,
    rx_errors   BIGINT,
    rx_drops    BIGINT,
    tx_bytes    BIGINT,
    tx_packets  BIGINT,
    tx_errors   BIGINT,
    tx_drops    BIGINT
);
SELECT create_hypertable('metrics_network', 'time');

-- Temperature metrics
CREATE TABLE metrics_temperature (
    time        TIMESTAMPTZ NOT NULL,
    agent_id    UUID NOT NULL,
    sensor      TEXT,
    Temperature DOUBLE PRECISION,
    max_temp    DOUBLE PRECISION
);
SELECT create_hypertable('metrics_temperature', 'time');

-- Wifi metrics
CREATE TABLE metrics_wifi (
    time            TIMESTAMPTZ NOT NULL,
    agent_id        UUID NOT NULL,
    interface       TEXT,
    ssid            TEXT,
    bssid           TEXT,
    frequency_mhz   INTEGER,
    signal_dbm      INTEGER,
    noise_dbm       INTEGER,
    bitrate_mbps    DOUBLE PRECISION
);
SELECT create_hypertable('metrics_wifi', 'time');

-- System metrics
CREATE TABLE metrics_system (
    time            TIMESTAMPTZ NOT NULL,
    agent_id        UUID NOT NULL,
    uptime          BIGINT,
    process_count   INTEGER,
    user_count      INTEGER,
    boot_time       BIGINT
);
SELECT create_hypertable('metrics_system', 'time');

-- Container metrics
CREATE TABLE metrics_container (
    time            TIMESTAMPTZ NOT NULL,
    agent_id        UUID NOT NULL,
    container_id    TEXT,
    name            TEXT,
    image           TEXT,
    state           TEXT,
    source          TEXT,
    kind            TEXT,
    cpu_percent     DOUBLE PRECISION,
    cpu_cores       INTEGER,
    memory_bytes    BIGINT,
    memory_limit    BIGINT,
    net_rx_bytes    BIGINT,
    net_tx_bytes    BIGINT
);
SELECT create_hypertable('metrics_container', 'time');

-- Pi-specific metrics
CREATE TABLE metrics_pi (
    time            TIMESTAMPTZ NOT NULL,
    agent_id        UUID NOT NULL,
    metric_type     TEXT NOT NULL,
    arm_freq_hz     BIGINT,
    core_freq_hz    BIGINT,
    gpu_freq_hz     BIGINT,
    core_volts      DOUBLE PRECISION,
    sdram_c_volts   DOUBLE PRECISION,
    sdram_i_volts   DOUBLE PRECISION,
    sdram_p_volts   DOUBLE PRECISION,
    throttled       BOOLEAN,
    under_voltage   BOOLEAN,
    freq_capped     BOOLEAN,
    gpu_mem_total   BIGINT,
    gpu_mem_used    BIGINT,
    gpu_temp        DOUBLE PRECISION
);
SELECT create_hypertable('metrics_pi', 'time');