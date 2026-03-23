-- name: GetAgentConfig :many
SELECT agent_id, config_key, config_value, updated_at
FROM agent_config
WHERE agent_id = @agent_id
ORDER BY config_key;

-- name: GetAgentConfigByKey :one
SELECT agent_id, config_key, config_value, updated_at
FROM agent_config
WHERE agent_id = @agent_id AND config_key = @config_key;

-- name: SetAgentConfig :exec
INSERT INTO agent_config (agent_id, config_key, config_value, updated_at)
VALUES (@agent_id, @config_key, @config_value, NOW())
ON CONFLICT (agent_id, config_key)
DO UPDATE SET config_value = @config_value, updated_at = NOW();

-- name: DeleteAgentConfig :exec
DELETE FROM agent_config
WHERE agent_id = @agent_id AND config_key = @config_key;

-- name: DeleteAllAgentConfig :exec
DELETE FROM agent_config
WHERE agent_id = @agent_id;