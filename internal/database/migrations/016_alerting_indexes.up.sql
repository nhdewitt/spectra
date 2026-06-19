DROP INDEX IF EXISTS idx_alert_events_active;
 
CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_events_one_active_per_rule_agent
    ON alert_events (rule_id, agent_id)
    WHERE resolved_at IS NULL;
 
-- 2. Cooldown / history: most-recent-event-per-pair lookups and the
--    ListLastEventPerRuleAgent DISTINCT ON query.
CREATE INDEX IF NOT EXISTS idx_alert_events_rule_agent_fired
    ON alert_events (rule_id, agent_id, fired_at DESC);
 
-- 3. Disk-trend query: WHERE agent_id = .. AND mountpoint = .. AND time >= ..
--    ORDER BY time ASC. TimescaleDB creates a time index on the hypertable but
--    not this composite.
CREATE INDEX IF NOT EXISTS idx_metrics_disk_agent_mount_time
    ON metrics_disk (agent_id, mountpoint, time);