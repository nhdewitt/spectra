package server

import (
	"context"
	"fmt"

	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/labels"
)

// Auto-label sync.
//
// versionCache (on Server) gates DB writes so the metrics hot path doesn't
// hit Postgres on every request. After a server restart the cache
// repopulates lazily as agents make their first authenticated request -
// by design, this triggers a single re-verification per agent on cold start.
//

// syncAutoLabelsOnRegister performs a full auto-label sync for a
// (re-)registered agent. Call after the agent row has been upserted.
//
// Idempotent: ReplaceAutoLabels IS DISTINCT FROM guard makes a re-sync
// with identical info a no-op at the DB level.
func (s *Server) syncAutoLabelsOnRegister(ctx context.Context, agentID string, info labels.AgentInfo) error {
	autoLabels := labels.ComputeAutoLabels(info)
	keys, values := labels.Unpack(autoLabels)

	err := s.DB.ReplaceAutoLabels(ctx, database.ReplaceAutoLabelsParams{
		AgentID: mustUUID(agentID),
		Keys:    keys,
		Values:  values,
	})
	if err != nil {
		return fmt.Errorf("replace auto labels for %s: %w", agentID, err)
	}

	if info.AgentVersion != "" {
		s.versionCache.Update(agentID, info.AgentVersion)
	}
	s.Logger.Debug("auto labels synced on register",
		"agent_id", agentID,
		"count", len(autoLabels))

	return nil
}

// syncAgentVersionLabel is the metrics-handler hot path. Checks the version
// cache and, on drift only, upserts the agent_version label without
// disturbing the agent's other auto labels.
//
// Returns nil for empty version and for no-drift. Errors should be logged
// at the call site but should not fail the metrics request - the cache is
// only updated on success, so the next upload retries automatically.
func (s *Server) syncAgentVersionLabel(ctx context.Context, agentID, version string) error {
	if !s.versionCache.Changed(agentID, version) {
		return nil
	}

	err := s.DB.UpsertAutoLabel(ctx, database.UpsertAutoLabelParams{
		AgentID: mustUUID(agentID),
		Key:     "agent_version",
		Value:   version,
	})
	if err != nil {
		return fmt.Errorf("upsert agent_version for %s: %w", agentID, err)
	}

	s.versionCache.Update(agentID, version)
	s.Logger.Debug("agent_version label updated",
		"agent_id", agentID,
		"version", version)

	return nil
}

// forgetAgentLabels drops the agent from the version cache. Call from the
// agent-delete path so a re-registration with the same UUID doesn't see a
// stale cache hit and skip its initial sync.
func (s *Server) forgetAgentLabels(agentID string) {
	s.versionCache.Forget(agentID)
}
