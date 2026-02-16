-- Retention policies: drop data older than 30 days
SELECT add_retention_policy('metrics_cpu', INTERVAL '30 days');
SELECT add_retention_policy('metrics_memory', INTERVAL '30 days');
SELECT add_retention_policy('metrics_disk', INTERVAL '30 days');
SELECT add_retention_policy('metrics_disk_io', INTERVAL '30 days');
SELECT add_retention_policy('metrics_network', INTERVAL '30 days');
SELECT add_retention_policy('metrics_temperature', INTERVAL '30 days');
SELECT add_retention_policy('metrics_wifi', INTERVAL '30 days');
SELECT add_retention_policy('metrics_system', INTERVAL '30 days');
SELECT add_retention_policy('metrics_container', INTERVAL '30 days');
SELECT add_retention_policy('metrics_pi', INTERVAL '30 days');

-- Indexes for common query patterns: "show me agent X's metrics over time"
CREATE INDEX idx_cpu_agent ON metrics_cpu (agent_id, time DESC);
CREATE INDEX idx_memory_agent ON metrics_memory (agent_id, time DESC);
CREATE INDEX idx_disk_agent ON metrics_disk (agent_id, time DESC);
CREATE INDEX idx_disk_io_agent ON metrics_disk_io (agent_id, time DESC);
CREATE INDEX idx_network_agent ON metrics_network (agent_id, time DESC);
CREATE INDEX idx_temperature_agent ON metrics_temperature (agent_id, time DESC);
CREATE INDEX idx_wifi_agent ON metrics_wifi (agent_id, time DESC);
CREATE INDEX idx_system_agent ON metrics_system (agent_id, time DESC);
CREATE INDEX idx_container_agent ON metrics_container (agent_id, time DESC);
CREATE INDEX idx_pi_agent ON metrics_pi (agent_id, time DESC);

-- Current-state indexes for dashboard queries
CREATE INDEX idx_processes_cpu ON current_processes (agent_id, cpu_percent DESC);
CREATE INDEX idx_processes_mem ON current_processes (agent_id, mem_rss DESC);
CREATE INDEX idx_services_status ON current_services (agent_id, status);

-- Token cleanup: find expired tokens
CREATE INDEX idx_tokens_expires ON registration_tokens (expires_at) WHERE NOT used;