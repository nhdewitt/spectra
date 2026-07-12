-- Indexes supporting the paginated overview query (server-side filtering).
-- These accelerate the WHERE filters and the sargable last_seen predicate; they
-- do not attempt to serve the ORDER BY (which sorts the classified CTE result,
-- not base columns, so the planner sorts regardless). Hostname substring search
-- (ILIKE '%..%') is intentionally left unindexed — a leading wildcard can't use
-- a btree, and a trigram index is premature at current fleet sizes.

-- Equality filters.
CREATE INDEX IF NOT EXISTS idx_agents_os ON agents (os);
CREATE INDEX IF NOT EXISTS idx_agents_arch ON agents (arch);

-- Sargable last_seen predicate (status classification) and last_seen sort.
CREATE INDEX IF NOT EXISTS idx_agents_last_seen ON agents (last_seen);

-- Label EXISTS probes: each starts from agent_id, then key/value. Leading
-- agent_id matches the correlation. If a unique index on (agent_id, key) already
-- exists from the ON CONFLICT (agent_id, key) constraint, this extends coverage
-- to value for index-only EXISTS checks.
CREATE INDEX IF NOT EXISTS idx_agent_labels_lookup ON agent_labels (agent_id, key, value);