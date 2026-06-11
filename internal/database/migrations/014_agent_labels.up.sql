-- agent_labels
--
-- Shared key/value metadata about agents, used for filtering in alert rules,
-- fleet queries, and the UI.
--
-- Two sources:
-- 	source='auto' :	computed server-side from agent registration/heartbeat
--					data (os, arch, platform, agent_version). Users cannot
--					modify these.
--	source='user' :	free-form labels set by admins via the API.
--
-- Labels are shared across all users. For per-user filtering preferences, see
-- user_config / future saved-views feature.
--
-- Reserved keys are enforced in application code, not the DB, since the list
-- of auto labels evolves with the codebase.

CREATE TABLE IF NOT EXISTS agent_labels (
	agent_id	UUID		NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
	key			TEXT		NOT NULL,
	value		TEXT		NOT NULL,
	source		TEXT		NOT NULL CHECK (source IN ('auto', 'user')),
	updated_at	TIMESTAMPTZ	NOT NULL DEFAULT NOW(),
	PRIMARY KEY (agent_id, key)
);

-- Lookups by label (alert rule evaluation, UI filter chips, fleet breakdown):
-- 	WHERE key = 'env' AND value = 'prod'
--	WHERE key = 'platform'
CREATE INDEX IF NOT EXISTS idx_agent_labels_key_value
	ON agent_labels (key, value);