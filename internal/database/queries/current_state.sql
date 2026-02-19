-- name: UpsertProcess :exec
INSERT INTO current_processes (agent_id, pid, name, cpu_percent, mem_percent, mem_rss, status, threads, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
ON CONFLICT (agent_id, pid) DO UPDATE
SET name = EXCLUDED.name,
    cpu_percent = EXCLUDED.cpu_percent,
    mem_percent = EXCLUDED.mem_percent,
    mem_rss = EXCLUDED.mem_rss,
    status = EXCLUDED.status,
    threads = EXCLUDED.threads,
    updated_at = NOW();

-- name: DeleteStaleProcesses :exec
DELETE FROM current_processes
WHERE agent_id = $1 AND updated_at < $2;

-- name: GetProcessesByCPU :many
SELECT agent_id, pid, name, cpu_percent, mem_percent, mem_rss, status, threads, updated_at
FROM current_processes
WHERE agent_id = $1
ORDER BY cpu_percent DESC
LIMIT $2;

-- name: GetProcessesByMemory :many
SELECT agent_id, pid, name, cpu_percent, mem_percent, mem_rss, status, threads, updated_at
FROM current_processes
WHERE agent_id = $1
ORDER BY mem_rss DESC
LIMIT $2;

-- name: UpsertService :exec
INSERT INTO current_services (agent_id, name, status, sub_status, updated_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (agent_id, name) DO UPDATE
SET status = EXCLUDED.status,
    sub_status = EXCLUDED.sub_status,
    updated_at = NOW();

-- name: GetServices :many
SELECT agent_id, name, status, sub_status, updated_at
FROM current_services
WHERE agent_id = $1
ORDER BY name;

-- name: UpsertApplication :exec
INSERT INTO current_applications (agent_id, name, version, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (agent_id, name) DO UPDATE
SET version = EXCLUDED.version,
    updated_at = NOW();

-- name: GetApplications :many
SELECT agent_id, name, version, updated_at
FROM current_applications
WHERE agent_id = $1
ORDER BY name;

-- name: UpsertUpdates :exec
INSERT INTO current_updates (agent_id, pending_count, security_count, reboot_required, package_manager, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (agent_id) DO UPDATE
SET pending_count = EXCLUDED.pending_count,
    security_count = EXCLUDED.security_count,
    reboot_required = EXCLUDED.reboot_required,
    package_manager = EXCLUDED.package_manager,
    updated_at = NOW();

-- name: GetUpdates :one
SELECT agent_id, pending_count, security_count, reboot_required, package_manager, updated_at
FROM current_updates
WHERE agent_id = $1;