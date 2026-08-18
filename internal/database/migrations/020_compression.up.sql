-- Enable TimescaleDB native compression on the metrics hypertables.

ALTER TABLE metrics_cpu SET (
	timescaledb.compress,
	timescaledb.compress_segmentby = 'agent_id',
	timescaledb.compress_orderby = 'time DESC'
);

ALTER TABLE metrics_memory SET (
	timescaledb.compress,
	timescaledb.compress_segmentby = 'agent_id',
	timescaledb.compress_orderby = 'time DESC'
);

ALTER TABLE metrics_system SET (
	timescaledb.compress,
	timescaledb.compress_segmentby = 'agent_id',
	timescaledb.compress_orderby = 'time DESC'
);
 
ALTER TABLE metrics_pi SET (
	timescaledb.compress,
	timescaledb.compress_segmentby = 'agent_id',
	timescaledb.compress_orderby = 'time DESC'
);
 
ALTER TABLE metrics_disk SET (
	timescaledb.compress,
	timescaledb.compress_segmentby = 'agent_id, device',
	timescaledb.compress_orderby = 'time DESC'
);
 
ALTER TABLE metrics_disk_io SET (
	timescaledb.compress,
	timescaledb.compress_segmentby = 'agent_id, device',
	timescaledb.compress_orderby = 'time DESC'
);
 
ALTER TABLE metrics_network SET (
	timescaledb.compress,
	timescaledb.compress_segmentby = 'agent_id, interface',
	timescaledb.compress_orderby = 'time DESC'
);
 
ALTER TABLE metrics_wifi SET (
	timescaledb.compress,
	timescaledb.compress_segmentby = 'agent_id, interface',
	timescaledb.compress_orderby = 'time DESC'
);
 
ALTER TABLE metrics_temperature SET (
	timescaledb.compress,
	timescaledb.compress_segmentby = 'agent_id, sensor',
	timescaledb.compress_orderby = 'time DESC'
);
 
ALTER TABLE metrics_container SET (
	timescaledb.compress,
	timescaledb.compress_segmentby = 'agent_id, container_id',
	timescaledb.compress_orderby = 'time DESC'
);
 
-- Compress chunks older than 7 days. Retention drops at 30 days, so this
-- leaves a 7-day uncompressed window well clear of the agent write path.
-- if_not_exists keeps the file replayable.
SELECT add_compression_policy(t, INTERVAL '7 days', if_not_exists => TRUE)
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