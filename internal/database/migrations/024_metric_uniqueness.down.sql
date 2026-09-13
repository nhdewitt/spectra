-- Drop the uniqueness indexes. Duplicate rows removed by the up migration are
-- not recoverable, which is the intended one-way part: they were never
-- meaningful data.
DROP INDEX IF EXISTS metrics_cpu_unique;
DROP INDEX IF EXISTS metrics_memory_unique;
DROP INDEX IF EXISTS metrics_system_unique;
DROP INDEX IF EXISTS metrics_pi_unique;
DROP INDEX IF EXISTS metrics_disk_unique;
DROP INDEX IF EXISTS metrics_disk_io_unique;
DROP INDEX IF EXISTS metrics_network_unique;
DROP INDEX IF EXISTS metrics_wifi_unique;
DROP INDEX IF EXISTS metrics_container_unique;