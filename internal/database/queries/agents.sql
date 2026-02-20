-- name: RegisterAgent :exec
INSERT INTO agents (id, secret_hash, hostname, os, platform, arch, cpu_model, cpu_cores, ram_total)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetAgent :one
SELECT id, secret_hash, hostname, os, platform, arch, cpu_model, cpu_cores, ram_total, registered_at, last_seen
FROM agents WHERE id = $1;

-- name: ListAgents :many
SELECT id, hostname, os, platform, arch, cpu_cores, ram_total, registered_at, last_seen
FROM agents
ORDER BY hostname;

-- name: UpdateLastSeen :exec
UPDATE agents SET last_seen = NOW() WHERE id = $1;

-- name: UpdateAgentInfo :exec
UPDATE agents
SET hostname = $2, os = $3, platform = $4, arch = $5, cpu_model = $6, cpu_cores = $7, ram_total = $8, last_seen = NOW()
WHERE id = $1;

-- name: DeleteAgent :exec
DELETE FROM agents WHERE id = $1;

-- name: AgentExists :one
SELECT EXISTS(SELECT 1 FROM agents WHERE id = $1);

-- name: GetAgentSecret :one
SELECT secret_hash FROM agents WHERE id = $1;

-- name: TouchLastSeen :exec
UPDATE agents SET last_seen = NOW() WHERE id = $1;