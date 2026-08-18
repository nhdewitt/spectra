-- Reverse 020_compression.
--
-- Order is load-bearing:
--   1. remove policies first, or the job recompresses mid-decompress
--   2. decompress every compressed chunk
--   3. only then can compression be disabled; SET (compress = false) errors
--      while any chunk is still compressed

SELECT remove_compression_policy(t, if_exists => TRUE)
FROM unnest(ARRAY[
	'metrics_cpu',
	'metrics_memory',
	'metrics_system',
	'metrics_pi',
	'metrics_disk',
	'metrics_disk_io',
	'metrics_network',
	'metrics_wifi',
	'metrics_temperature',
	'metrics_container'
]::regclass[]) t;

SELECT decompress_chunk(format('%I.%I', c.chunk_schema, c.chunk_name)::regclass, if_compressed => TRUE)
FROM timescaledb_information.chunks c
WHERE c.hypertable_schema = 'public'
  AND c.hypertable_name IN (
	'metrics_cpu',
	'metrics_memory',
	'metrics_system',
	'metrics_pi',
	'metrics_disk',
	'metrics_disk_io',
	'metrics_network',
	'metrics_wifi',
	'metrics_temperature',
	'metrics_container'
  )
  AND c.is_compressed;

ALTER TABLE metrics_cpu SET (timescaledb.compress = false);
ALTER TABLE metrics_memory SET (timescaledb.compress = false);
ALTER TABLE metrics_system SET (timescaledb.compress = false);
ALTER TABLE metrics_pi SET (timescaledb.compress = false);
ALTER TABLE metrics_disk SET (timescaledb.compress = false);
ALTER TABLE metrics_disk_io SET (timescaledb.compress = false);
ALTER TABLE metrics_network SET (timescaledb.compress = false);
ALTER TABLE metrics_wifi SET (timescaledb.compress = false);
ALTER TABLE metrics_temperature SET (timescaledb.compress = false);
ALTER TABLE metrics_container SET (timescaledb.compress = false);