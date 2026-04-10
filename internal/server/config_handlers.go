package server

import (
	"encoding/json"
	"net/http"

	"github.com/nhdewitt/spectra/internal/database"
)

// handleGetAgentConfig returns all config entries for an agent.
//
// GET /api/v1/agents/{id}/config
func (s *Server) handleGetAgentConfig(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rows, err := s.DB.GetAgentConfig(r.Context(), mustUUID(agentID))
	if err != nil {
		s.Logger.Error("database query failed", "error", err, "handler", "handleGetAgentConfig")
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	config := make(map[string]json.RawMessage, len(rows))
	for _, row := range rows {
		config[row.ConfigKey] = row.ConfigValue
	}

	respondJSON(w, http.StatusOK, config)
}

// handleSetAgentConfig sets a single config key for an agent.
// Expects JSON body: {"key": "ignored_filesystems", "value": ["nfs", "cifs"]}
//
// PUT /api/v1/agents/{id}/config
func (s *Server) handleSetAgentConfig(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	if !json.Valid(req.Value) {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if !isValidConfigKey(req.Key) {
		http.Error(w, "invalid config key", http.StatusBadRequest)
		return
	}

	if err := s.DB.SetAgentConfig(r.Context(), database.SetAgentConfigParams{
		AgentID:     mustUUID(agentID),
		ConfigKey:   req.Key,
		ConfigValue: req.Value,
	}); err != nil {
		s.Logger.Error("database query failed", "error", err, "handler", "handleSetAgentConfig")
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	s.Logger.Info("agent config updated", "agent_id", agentID, "key", req.Key)
	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteAgentConfig deletes a single config key for an agent.
//
// DELETE /api/v1/agents/{id}/config
func (s *Server) handleDeleteAgentConfig(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key query parameter is required", http.StatusBadRequest)
		return
	}

	if err := s.DB.DeleteAgentConfig(r.Context(), database.DeleteAgentConfigParams{
		AgentID:   mustUUID(agentID),
		ConfigKey: key,
	}); err != nil {
		s.Logger.Error("database query failed", "error", err, "handler", "handleDeleteAgentConfig")
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	s.Logger.Info("agent config deleted", "agent_id", agentID, "key", key)
	w.WriteHeader(http.StatusNoContent)
}

// Valid config keys
var validConfigKeys = map[string]struct{}{
	"ignored_filesystems": {},
	"ignored_interfaces":  {},
	"labels":              {},
	"log_level":           {},
}

func isValidConfigKey(key string) bool {
	_, ok := validConfigKeys[key]
	return ok
}
