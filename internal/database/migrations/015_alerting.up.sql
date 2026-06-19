-- alerting
--
-- alert_channels 	   : delivery tagets (email, webhook). Configured globally.
-- alert_rules    	   : conditions to evaludate on a background tick. Scope is
--						 'global' (all agents) or 'agent' (one agent).
--						 Per-agent rules override global rules for the same
--						 condition_type + condition_params combo.
-- alert_rule_channels : many-to-many join between rules and channels.
-- alert_events		   : fired alert history. resolved_at NULL = still active.

CREATE TABLE IF NOT EXISTS alert_channels (
	id			UUID		PRIMARY KEY DEFAULT gen_random_uuid(),
	name		TEXT		NOT NULL,
	type		TEXT		NOT NULL CHECK (type IN ('email', 'webhook')),
	config		JSONB		NOT NULL DEFAULT '{}',
	created_at	TIMESTAMPTZ	NOT NULL DEFAULT NOW()
);

-- config schema by type:
--	email:		{"to": "ops@example.com"}
--	webhook:	{"url": "https://hooks.example.com/spectra"}
--				POST body is a JSON AlertPayload.

CREATE TABLE IF NOT EXISTS alert_rules (
	id					UUID		PRIMARY KEY DEFAULT gen_random_uuid(),
	name				TEXT		NOT NULL,
	enabled				BOOLEAN		NOT NULL DEFAULT TRUE,
	scope				TEXT		NOT NULL CHECK (scope IN ('global', 'agent')),
	agent_id			UUID		REFERENCES agents(id) ON DELETE CASCADE,
	condition_type		TEXT		NOT NULL CHECK (condition_type IN (
										'agent_offline',
										'disk_prediction',
										'service_down'
									)),
	condition_params	JSONB		NOT NULL DEFAULT '{}',
	cooldown_seconds	INTEGER		NOT NULL DEFAULT 3600,
	created_at			TIMESTAMPTZ	NOT NULL DEFAULT NOW(),
	updated_at			TIMESTAMPTZ	NOT NULL DEFAULT NOW(),

	-- global rules must not reference an agent; agent rules must
	CONSTRAINT scope_agent_consistent CHECK (
		(scope = 'global' AND agent_id IS NULL) OR
		(scope = 'agent' AND agent_id IS NOT NULL)
	)
);

-- condition params schema by condition_type:
--	agent_offline:		{"timeout_seconds": 300}
--	disk_prediction:	{"mount": "/var", "warn_hours": 72}
--						Fires when linear extrapolation predicts the mount will
--						be full within warn_hours based on recent trend.
--	service_down:		{"service_name": "nginx"}

CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled_type
	ON alert_rules (enabled, condition_type);

CREATE TABLE IF NOT EXISTS alert_rule_channels (
	rule_id		UUID	NOT NULL REFERENCES alert_rules(id)	ON DELETE CASCADE,
	channel_id	UUID	NOT NULL REFERENCES alert_channels(id) ON DELETE CASCADE,
	PRIMARY KEY (rule_id, channel_id)
);

CREATE TABLE IF NOT EXISTS alert_events (
	id					UUID		PRIMARY KEY DEFAULT gen_random_uuid(),
	rule_id				UUID		NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
	agent_id			UUID		NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
	fired_at			TIMESTAMPTZ	NOT NULL DEFAULT NOW(),
	resolved_at			TIMESTAMPTZ,
	last_notified_at	TIMESTAMPTZ,
	condition_snapshot	JSONB		NOT NULL DEFAULT '{}'
);

-- condition_snapshot examples:
--	agent_offline:		{"last_seen": "2026-06-15T12:00:00Z", "seconds_silent": 420}
--	disk_prediction:	{"mount": "/var", "used_pct": 87.3, "predicted_full_at": "2026-06-17T12:00:00Z", "hours_remaining": 48}
--	service_down:		{"service_name": "nginx", "last_status": "failed"}

CREATE INDEX IF NOT EXISTS idx_alert_events_active
	ON alert_events (rule_id, agent_id)
	WHERE resolved_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_alert_events_agent_id
	ON alert_events (agent_id, fired_at DESC);