-- Make metric ingest idempotent.
--
-- A duplicate delivery is possible whenever a chunk is written into a socket
-- the kernel later transmits. The agent's Client.Timeout fires at 45s, the
-- chunk is requeued and eventually re-sent, but the original bytes are still
-- in the send buffer being retransmitted until tcp_retries2 gives up (roughly
-- 15min at the default of 15). If the path recovers inside that window the
-- server recieves both copies. The inserts carry no conflict handling and the
-- tables no uniquenness, so that produced silent duplicate rows.
--
-- The keys are the natural identity of a sample, taken from each table's
-- compress_segmentby plus time. metrics_pi is the exception. It writes four
-- rows per collection (clock, voltage, throttle, gpu) sharing a timestamp, so
-- metric_type is part of its key even through it is not a segmentby column.
--
-- metrics_temperature is deliberately excluded. Its sensor name comes straight
-- from thermal_zoneN/type, which is "acpitz" for every ACPI zone, so a host
-- with two zones has been writing two distinct series under one name. Indexing
-- (agent_id, sensor, time) would delete one zone's readings on every such host,
-- and the historical rows cannot be disambiguated after the fact. The collector
-- now suffixes colliding names with the zone number. The index will be added in
-- a later migration once the ambiguous rows have aged past the 30-day retention
-- window.
--
-- TimescaleDB requires a unique index on a hypertable to include every
-- partitioning column. All ten are partitioned on time alone, so each key ends
-- with time.

-- Decompress every chunk that is currently compressed. if_compressed >= TRUE
-- makes this a no-op for chunks that are not, so the migration is safe to run
-- against a fresh install where nothing has been compressed yet.
SELECT decompress_chunk(c, if_compressed => TRUE)
FROM unnest(ARRAY[
	'metrics_cpu', 'metrics_memory', 'metrics_disk', 'metrics_disk_io',
	'metrics_network', 'metrics_wifi',
	'metrics_system', 'metrics_container', 'metrics_pi'
]::text[]) as t,
LATERAL show_chunks(t::regclass) AS c;

-- Remove existing duplicates, keeping one row per key.
--
-- ctid is only unique within a physical table, but rows sharing a timestamp
-- share a chunk, and every join below equates time, so the comparison never
-- spans chunks.
DELETE FROM metrics_cpu a USING metrics_cpu b
WHERE a.ctid < b.ctid AND a.agent_id = b.agent_id AND a.time = b.time;
 
DELETE FROM metrics_memory a USING metrics_memory b
WHERE a.ctid < b.ctid AND a.agent_id = b.agent_id AND a.time = b.time;
 
DELETE FROM metrics_system a USING metrics_system b
WHERE a.ctid < b.ctid AND a.agent_id = b.agent_id AND a.time = b.time;
 
DELETE FROM metrics_pi a USING metrics_pi b
WHERE a.ctid < b.ctid AND a.agent_id = b.agent_id AND a.metric_type = b.metric_type
  AND a.time = b.time;
 
DELETE FROM metrics_disk a USING metrics_disk b
WHERE a.ctid < b.ctid AND a.agent_id = b.agent_id AND a.device = b.device
  AND a.time = b.time;
 
DELETE FROM metrics_disk_io a USING metrics_disk_io b
WHERE a.ctid < b.ctid AND a.agent_id = b.agent_id AND a.device = b.device
  AND a.time = b.time;
 
DELETE FROM metrics_network a USING metrics_network b
WHERE a.ctid < b.ctid AND a.agent_id = b.agent_id AND a.interface = b.interface
  AND a.time = b.time;
 
DELETE FROM metrics_wifi a USING metrics_wifi b
WHERE a.ctid < b.ctid AND a.agent_id = b.agent_id AND a.interface = b.interface
  AND a.time = b.time;
 
DELETE FROM metrics_container a USING metrics_container b
WHERE a.ctid < b.ctid AND a.agent_id = b.agent_id AND a.container_id = b.container_id
  AND a.time = b.time;

-- Unique indexes. Not CONCURRENTLY (the migration running executes each file as
-- a single Exec, which puts these in an implicit transaction, and CREATE INDEX
-- CONCURRENTLY cannot run inside one). Blocking index builds are fine here because
-- migrations run before the server starts serving, so nothing else is writing to
-- these tables.
CREATE UNIQUE INDEX IF NOT EXISTS metrics_cpu_unique
    ON metrics_cpu (agent_id, time);
CREATE UNIQUE INDEX IF NOT EXISTS metrics_memory_unique
    ON metrics_memory (agent_id, time);
CREATE UNIQUE INDEX IF NOT EXISTS metrics_system_unique
    ON metrics_system (agent_id, time);
CREATE UNIQUE INDEX IF NOT EXISTS metrics_pi_unique
    ON metrics_pi (agent_id, metric_type, time);
CREATE UNIQUE INDEX IF NOT EXISTS metrics_disk_unique
    ON metrics_disk (agent_id, device, time);
CREATE UNIQUE INDEX IF NOT EXISTS metrics_disk_io_unique
    ON metrics_disk_io (agent_id, device, time);
CREATE UNIQUE INDEX IF NOT EXISTS metrics_network_unique
    ON metrics_network (agent_id, interface, time);
CREATE UNIQUE INDEX IF NOT EXISTS metrics_wifi_unique
    ON metrics_wifi (agent_id, interface, time);
CREATE UNIQUE INDEX IF NOT EXISTS metrics_container_unique
    ON metrics_container (agent_id, container_id, time);