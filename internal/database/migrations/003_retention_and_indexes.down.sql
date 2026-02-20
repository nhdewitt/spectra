-- Remove retention policies
SELECT remove_retention_policy('metrics_cpu', if_exists => true);
SELECT remove_retention_policy('metrics_memory', if_exists => true);
SELECT remove_retention_policy('metrics_disk', if_exists => true);
SELECT remove_retention_policy('metrics_disk_io', if_exists => true);
SELECT remove_retention_policy('metrics_network', if_exists => true);
SELECT remove_retention_policy('metrics_temperature', if_exists => true);
SELECT remove_retention_policy('metrics_wifi', if_exists => true);
SELECT remove_retention_policy('metrics_system', if_exists => true);
SELECT remove_retention_policy('metrics_container', if_exists => true);
SELECT remove_retention_policy('metrics_pi', if_exists => true);

-- Drop time-series indexes
DROP INDEX IF EXISTS idx_cpu_agent;
DROP INDEX IF EXISTS idx_memory_agent;
DROP INDEX IF EXISTS idx_disk_agent;
DROP INDEX IF EXISTS idx_disk_io_agent;
DROP INDEX IF EXISTS idx_network_agent;
DROP INDEX IF EXISTS idx_temperature_agent;
DROP INDEX IF EXISTS idx_wifi_agent;
DROP INDEX IF EXISTS idx_system_agent;
DROP INDEX IF EXISTS idx_container_agent;
DROP INDEX IF EXISTS idx_pi_agent;

-- Drop current-state indexes
DROP INDEX IF EXISTS idx_processes_cpu;
DROP INDEX IF EXISTS idx_processes_mem;
DROP INDEX IF EXISTS idx_services_status;

-- Drop token index
DROP INDEX IF EXISTS idx_tokens_expires;