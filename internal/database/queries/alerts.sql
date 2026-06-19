-- name: CreateAlertChannel :one
INSERT INTO alert_channels (name, type, config)
VALUES (@name, @type, @config)
RETURNING *;

-- name: GetAlertChannel :one
SELECT * FROM alert_channels
WHERE id = @id;

-- name: ListAlertChannels :many
SELECT * FROM alert_channels
ORDER BY name;

-- name: UpdateAlertChannel :one
UPDATE alert_channels
SET	name = @name,
	type = @type,
	config = @config
WHERE id = @id
RETURNING *;

-- name: DeleteAlertChannel :exec
DELETE FROM alert_channels WHERE id = @id;

-- name: CreateAlertRule :one
INSERT INTO alert_rules (name, enabled, scope, agent_id, condition_type, condition_params, cooldown_seconds)
VALUES (@name, @enabled, @scope, @agent_id, @condition_type, @condition_params, @cooldown_seconds)
RETURNING *;

-- name: GetAlertRule :one
SELECT * FROM alert_rules WHERE id = @id;

-- name: ListAlertRules :many
SELECT * FROM alert_rules
ORDER BY created_at DESC;

-- name: ListEnabledAlertRules :many
SELECT * FROM alert_rules
WHERE enabled = TRUE
ORDER BY scope DESC, created_at DESC;

-- name: UpdateAlertRule :one
UPDATE alert_rules
SET	name = @name,
	enabled = @enabled,
	condition_params = @condition_params,
	cooldown_seconds = @cooldown_seconds,
	updated_at = NOW()
WHERE id = @id
RETURNING *;

-- name: DeleteAlertRule :exec
DELETE FROM alert_rules WHERE id = @id;

-- name: SetAlertRuleEnabled :one
UPDATE alert_rules
SET	enabled = @enabled,
	updated_at = NOW()
WHERE id = @id
RETURNING *;

-- name: AddChannelToRule :exec
INSERT INTO alert_rule_channels (rule_id, channel_id)
VALUES (@rule_id, @channel_id)
ON CONFLICT DO NOTHING;

-- name: RemoveChannelFromRule :exec
DELETE FROM alert_rule_channels
WHERE rule_id = @rule_id AND channel_id = @channel_id;

-- name: ListChannelsForRule :many
SELECT ac.*
FROM alert_channels ac
JOIN alert_rule_channels arc ON arc.channel_id = ac.id
WHERE arc.rule_id = @rule_id
ORDER BY ac.name;

-- name: GetActiveEvent :one
-- Returns the open (unresolved) event (if any) for a rule+agent pair.
SELECT * FROM alert_events
WHERE rule_id = @rule_id
  AND agent_id = @agent_id
  AND resolved_at IS NULL
LIMIT 1;

-- name: GetLastEventForRule :one
-- Used for cooldown check: returns the most recently fired event
-- regardless of resolved state.
SELECT * FROM alert_events
WHERE rule_id = @rule_id
  AND agent_id = @agent_id
ORDER BY fired_at DESC
LIMIT 1;

-- name: CreateAlertEvent :one
INSERT INTO alert_events (rule_id, agent_id, condition_snapshot, last_notified_at)
VALUES (@rule_id, @agent_id, @condition_snapshot, NOW())
RETURNING *;

-- name: ResolveAlertEvent :exec
UPDATE alert_events
SET resolved_at = NOW()
WHERE rule_id = @rule_id
  AND agent_id = @agent_id
  AND resolved_at IS NULL;

-- name: TouchAlertEventNotified :exec
UPDATE alert_events
SET last_notified_at = NOW()
WHERE id = @id;

-- name: ListActiveAlertEvents :many
SELECT
	ae.*,
	ar.name AS rule_name,
	ar.condition_type,
	a.hostname
FROM alert_events ae
JOIN alert_rules ar ON ar.id = ae.rule_id
JOIN agents a ON a.id = ae.agent_id
WHERE ae.resolved_at IS NULL
ORDER BY ae.fired_at DESC;

-- name: ListAlertEventHistory :many
SELECT
	ae.*,
	ar.name AS rule_name,
	ar.condition_type,
	a.hostname
FROM alert_events ae
JOIN alert_rules ar ON ar.id = ae.rule_id
JOIN agents a ON a.id = ae.agent_id
ORDER BY ae.fired_at DESC
LIMIT $1 OFFSET $2;

-- name: ListAlertEventsByAgent :many
SELECT
	ae.*,
	ar.name AS rule_name,
	ar.condition_type
FROM alert_events ae
JOIN alert_rules ar ON ar.id = ae.rule_id
WHERE ae.agent_id = @agent_id
ORDER BY ae.fired_at DESC
LIMIT $1 OFFSET $2;

-- name: DeleteChannelsForRule :exec
DELETE FROM alert_rule_channels WHERE rule_id = @rule_id;

-- name: ListAllActiveEvents :many
-- Bulk-load every unresolved event so the evaluator can key them in Go by
-- rule_id + agent_id instead of issuing GetActiveEvent per rule/agent pair.
SELECT id, rule_id, agent_id, fired_at, resolved_at, last_notified_at, condition_snapshot
FROM alert_events
WHERE resolved_at IS NULL;
 
-- name: ListLastEventPerRuleAgent :many
-- Bulk-load the most recent event (resolved or not) for every rule+agent pair,
-- for cooldown checks. DISTINCT ON returns one row per (rule_id, agent_id),
-- ordered so the newest fired_at wins.
SELECT DISTINCT ON (rule_id, agent_id)
    id, rule_id, agent_id, fired_at, resolved_at, last_notified_at, condition_snapshot
FROM alert_events
ORDER BY rule_id, agent_id, fired_at DESC;