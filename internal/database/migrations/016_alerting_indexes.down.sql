DROP INDEX IF EXISTS idx_metrics_disk_agent_mount_time;
DROP INDEX IF EXISTS idx_alert_events_rule_agent_fired;
DROP INDEX IF EXISTS idx_alert_events_one_active_per_rule_agent;
 
CREATE INDEX IF NOT EXISTS idx_alert_events_active
    ON alert_events (rule_id, agent_id)
    WHERE resolved_at IS NULL;