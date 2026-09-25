-- name: UpsertProcesses :exec
-- One statement per process list. DISTINCT ON drops a repeated pid, which ON CONFLICT DO UPDATE
-- would reject, failing the batch on every retry.
INSERT INTO current_processes (agent_id, pid, name, cpu_percent, mem_percent, mem_rss, status, threads, updated_at)
SELECT DISTINCT ON (p.pid) @agent_id::uuid, p.pid, p.name, p.cpu_percent, p.mem_percent, p.mem_rss, p.status, p.threads, NOW()
FROM (
    SELECT  unnest(@pids::integer[]) AS pid,
            unnest(@names::text[]) AS name,
            unnest(@cpu_percents::double precision[]) AS cpu_percent,
            unnest(@mem_percents::double precision[]) AS mem_percent,
            unnest(@mem_rss::bigint[]) AS mem_rss,
            unnest(@statuses::text[]) AS status,
            unnest(@threads::integer[]) AS threads
) AS p
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

-- name: UpsertServices :exec
-- One statement per service list; DISTINCT ON as in UpsertProcesses
INSERT INTO current_services (agent_id, name, status, sub_status, updated_at)
SELECT DISTINCT ON (s.name) @agent_id::uuid, s.name, s.status, s.sub_status, NOW()
FROM (
    SELECT  unnest(@names::text[]) AS name,
            unnest(@statuses::text[]) AS status,
            unnest(@sub_statuses::text[]) AS sub_status
) AS s
ON CONFLICT (agent_id, name) DO UPDATE
SET status = EXCLUDED.status,
    sub_status = EXCLUDED.sub_status,
    updated_at = NOW();

-- name: GetServices :many
SELECT agent_id, name, status, sub_status, updated_at
FROM current_services
WHERE agent_id = $1
ORDER BY name;

-- name: UpsertApplications :exec
-- One statement per application list; DISTINCT ON as in UpsertProcesses
INSERT INTO current_applications (agent_id, name, version, updated_at)
SELECT DISTINCT ON (a.name) @agent_id::uuid, a.name, a.version, NOW()
FROM (
    SELECT  unnest(@names::text[]) AS name,
            unnest(@versions::text[]) AS version
) AS a
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

-- name: GetAllServices :many
-- Bulk-load current services across the whole fleet so the evaluator can build
-- an agent_id -> services map in one query instead of GetServices per agent.
SELECT agent_id, name, status, sub_status, updated_at
FROM current_services
ORDER BY agent_id, name;