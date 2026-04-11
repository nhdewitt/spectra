-- name: UpdateAgentVersion :exec
UPDATE agents
SET version = @version
WHERE id = @id
    AND (version IS DISTINCT FROM @version);