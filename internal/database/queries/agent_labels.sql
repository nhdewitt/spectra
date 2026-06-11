-- agent_labels
--
-- Conventions:
--	- User writes go through UpsertUserLabel / DeleteUserLabel.
--	- Auto-label sync goes through ReplaceAutoLabels (one call per
--	  agent register / heartbeat; idempotent).
--	- The ON CONFLICT WHERE clauses ensure auto and user labels can
--	  never clobber each other regardless of caller mistake.

-- name: ListAgentLabels :many
-- All labels for one agent. Auto labels listed first so the UI can group them.
SELECT key, value, source, updated_at
FROM agent_labels
WHERE agent_id = @agent_id
ORDER BY source DESC, key ASC;

-- name: GetAgentLabel :one
-- Single label lookup; used by the API to disambiguate 404 vs 403 (auto label
-- the caller tried to modify) before calling a write query.
SELECT key, value, source, updated_at
FROM agent_labels
WHERE agent_id = @agent_id AND key = @key;

-- name: ListAgentLabelsForAgents :many
-- Batch label fetch for fleet/overview pages. Returns one row per (agent_id,
-- key); caller groups by agent_id.
SELECT agent_id, key, value, source
FROM agent_labels
WHERE agent_id = ANY(@agent_ids::uuid[])
ORDER BY agent_id, source DESC, key ASC;

-- name: ListLabelKeys :many
-- Distinct keys across the fleet, for the rule-editor key picker.
-- Includes source so the UI can flag auto keys as read-only.
SELECT DISTINCT key, source
FROM agent_labels
ORDER BY source DESC, key ASC;

-- name: ListLabelValuesForKey :many
-- Distinct values for a key, for the rule-editor value autocomplete.
SELECT DISTINCT value
FROM agent_labels
WHERE key = @key
ORDER BY value ASC;

-- name: CountAgentsByLabelValue :many
-- Fleet breakdown by a single label key.
SELECT value, COUNT(*)::bigint AS count
FROM agent_labels
WHERE key = @key
GROUP BY value
ORDER BY count DESC, value ASC;

-- name: ListAgentsByLabel :many
-- Agents matching a single key=value pair.
SELECT agent_id
FROM agent_labels
WHERE key = @key and value = @value;

-- name: ListAgentsByLabels :many
-- Agents matching ALL of the given (key, value) pairs.
--
-- @keys and @values must be parallel arrays of equal length.
SELECT al.agent_id
FROM agent_labels AS al
JOIN (
    SELECT keys.k_val, vals.v_val
    FROM unnest(@keys::text[]) WITH ORDINALITY AS keys(k_val, idx)
    JOIN unnest(@values::text[]) WITH ORDINALITY AS vals(v_val, idx)
        USING (idx)
) AS f ON al.key = f.k_val AND al.value = f.v_val
GROUP BY al.agent_id
HAVING COUNT(*) = COALESCE(array_length(@keys::text[], 1), 0);

-- name: UpsertUserLabel :one
-- Admin-only write. The Go layer must reject reserved keys before
-- calling this - the reserved set lives in code, not the DB.
--
-- Returns NULL if no row was inserted/updated (caller should check).
INSERT INTO agent_labels (agent_id, key, value, source, updated_at)
VALUES (@agent_id, @key, @value, 'user', NOW())
ON CONFLICT (agent_id, key)
DO UPDATE SET
	value = EXCLUDED.value,
	updated_at = NOW()
WHERE agent_labels.source = 'user'
RETURNING agent_id, key, value, source, updated_at;

-- name: DeleteUserLabel :execrows
-- Returns rows affected. Zero means either not found or exists but
-- is auto-sourced; caller can disambiguate with GetAgentLabel.
DELETE FROM agent_labels
WHERE agent_id = @agent_id
	AND key = @key
	AND source = 'user';

-- name: ReplaceAutoLabels :exec
-- Idempotent sync of all auto labels for an agent in a single statement.
-- Called on agent registration and on detected version drift.
--
-- Behavior:
--	1. Auto labels that aren't in the new set are removed.
--	2. New auto labels are inserted.
--	3. Existing auto labels are updated if value changed.
--	4. User labels are never touched.
--
-- @keys and @values are parallel arrays. Empty arrays clear all auto labels
-- for the agent.
WITH new_labels AS (
    SELECT keys.k_val AS k, vals.v_val AS v
    FROM unnest(@keys::text[]) WITH ORDINALITY AS keys(k_val, idx)
    JOIN unnest(@values::text[]) WITH ORDINALITY AS vals(v_val, idx)
        USING (idx)
),
deleted AS (
    DELETE FROM agent_labels al
    WHERE al.agent_id = @agent_id
      AND al.source = 'auto'
      AND NOT EXISTS (
          SELECT 1 FROM new_labels nl WHERE nl.k = al.key
      )
    RETURNING 1
)
INSERT INTO agent_labels (agent_id, key, value, source, updated_at)
SELECT @agent_id, k, v, 'auto', NOW() FROM new_labels
ON CONFLICT (agent_id, key)
DO UPDATE SET
    value      = EXCLUDED.value,
    updated_at = NOW()
WHERE agent_labels.source = 'auto'
  AND agent_labels.value IS DISTINCT FROM EXCLUDED.value;

-- name: UpsertAutoLabel :exec
-- Single-label auto upsert for hot paths (metrics-handler version drift)
-- that update one label without re-syncing the whole set. Same guards as
-- ReplaceAutoLabels: never clobbers user labels, skips writes when value
-- is unchanged.
INSERT INTO agent_labels (agent_id, key, value, source, updated_at)
VALUES (@agent_id, @key, @value, 'auto', NOW())
ON CONFLICT (agent_id, key)
DO UPDATE SET
    value = EXCLUDED.value,
    updated_at = NOW()
WHERE agent_labels.source = 'auto'
    AND agent_labels.value IS DISTINCT FROM EXCLUDED.value;