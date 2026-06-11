package labels

import (
	"sync"
)

// VersionCache tracks the last-seen agent_version per agent so the metrics
// handler can call ReplaceAutoLabels only on actual drift, not every heartbeat.
//
// Behavior:
//   - In-memory only. A server restart causes one re-sync per agent on
//     their first authenticated request, which is the desired behavior.
//   - First sighting of an agent is treated as drift, ensuring the sync
//     runs on registration even if the cache was populated from a different
//     path first.
//   - Empty incoming version strings are ignored so pre-versioned agents during
//     a rolling upgrade don't trigger spurious syncs that would clear the
//     agent_version label.
type VersionCache struct {
	mu sync.RWMutex
	m  map[string]string
}

// NewVersionCache returns an empty cache ready for use.
func NewVersionCache() *VersionCache {
	return &VersionCache{m: make(map[string]string)}
}

// Changed reports whether the given (agent, version) pair differs from the
// cached value. Returns false (no drift) if version is empty - see the type
// comment for rationale. Does not update the cache; call Update after the
// auto-label sync succeeds so a failed sync gets retried on the next request.
func (c *VersionCache) Changed(agentID, version string) bool {
	if version == "" {
		return false
	}
	c.mu.RLock()
	prev, ok := c.m[agentID]
	c.mu.RUnlock()
	return !ok || prev != version
}

// Update records the version for the given agent. Call this only after a
// successful ReplaceAutoLabels - caching a version we failed to persist
// would mean the next request sees no drift and the DB stays stale.
func (c *VersionCache) Update(agentID, version string) {
	if version == "" {
		return
	}
	c.mu.Lock()
	c.m[agentID] = version
	c.mu.Unlock()
}

// Forget drops an agent from the cache. Call from the agent-delete path
// so a re-registered agent with the same UUID doesn't see a stale entry.
func (c *VersionCache) Forget(agentID string) {
	c.mu.Lock()
	delete(c.m, agentID)
	c.mu.Unlock()
}

// Len returns the number of agents currently cached.
func (c *VersionCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.m)
}
