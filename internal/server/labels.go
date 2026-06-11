package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/labels"
)

type labelDTO struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updated_at"`
}

type labelKeyDTO struct {
	Key    string `json:"key"`
	Source string `json:"source"`
}

type putLabelRequest struct {
	Value string `json:"value"`
}

// handleListAgentLabels returns all labels (auto + user) for one agent.
//
// GET /api/v1/agents/{id}/labels
func (s *Server) handleListAgentLabels(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	rows, err := s.DB.ListAgentLabels(r.Context(), mustUUID(agentID))
	if err != nil {
		s.dbError(w, err, "handleListAgentLabels")
		return
	}

	out := make([]labelDTO, len(rows))
	for i, row := range rows {
		out[i] = labelDTO{
			Key:       row.Key,
			Value:     row.Value,
			Source:    row.Source,
			UpdatedAt: row.UpdatedAt.Time,
		}
	}
	respondJSON(w, http.StatusOK, out)
}

// handleListLabelKeys returns the distinct set of label keys across the
// fleet. Used by the rule-editor key picker; source is included so the UI
// can flag auto keys as read-only.
//
// GET /api/v1/labels/keys
func (s *Server) handleListLabelKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.ListLabelKeys(r.Context())
	if err != nil {
		s.dbError(w, err, "handleListLabelKeys")
		return
	}

	out := make([]labelKeyDTO, len(rows))
	for i, row := range rows {
		out[i] = labelKeyDTO{Key: row.Key, Source: row.Source}
	}
	respondJSON(w, http.StatusOK, out)
}

// handleListLabelValues returns the distinct values for a given key. Used
// by the rule-editor value autocomplete.
//
// GET /api/v1/labels/values?key={key}
func (s *Server) handleListLabelValues(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		respondError(w, http.StatusBadRequest, "key query parameter is required")
		return
	}

	values, err := s.DB.ListLabelValuesForKey(r.Context(), key)
	if err != nil {
		s.dbError(w, err, "handleListLabelValues")
		return
	}

	// Normalize nil -> empty slicce so JSON is `[]` not `null`.
	if values == nil {
		values = []string{}
	}
	respondJSON(w, http.StatusOK, values)
}

// handlePutAgentLabel upserts a user-sourced label on an agent. Reserved
// keys (the auto-label namespace) and malformed keys/values are rejected.
//
// PUT /api/v1/admin/agents/{id}/labels/{key}
func (s *Server) handlePutAgentLabel(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := r.PathValue("key")

	var req putLabelRequest
	if err := decodeJSONBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := labels.ValidateUserLabel(key, req.Value); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, labels.ErrReservedKey) {
			status = http.StatusForbidden
		}
		respondError(w, status, err.Error())
		return
	}

	row, err := s.DB.UpsertUserLabel(r.Context(), database.UpsertUserLabelParams{
		AgentID: mustUUID(agentID),
		Key:     key,
		Value:   req.Value,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, http.StatusConflict, "label key is held by an auto label")
			return
		}
		s.dbError(w, err, "handlePutAgentLabel")
		return
	}

	respondJSON(w, http.StatusOK, labelDTO{
		Key:       row.Key,
		Value:     row.Value,
		Source:    row.Source,
		UpdatedAt: row.UpdatedAt.Time,
	})
}

// handleDeleteAgentLabel removes a user-sourced label from an agent.
// Auto-sourced labels cannot be deleted via this endpoint.
//
// DELETE /api/v1/admin/agents/{id}/labels/{key}
func (s *Server) handleDeleteAgentLabel(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseAgentID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := r.PathValue("key")

	rows, err := s.DB.DeleteUserLabel(r.Context(), database.DeleteUserLabelParams{
		AgentID: mustUUID(agentID),
		Key:     key,
	})
	if err != nil {
		s.dbError(w, err, "handleDeleteAgentLabel")
		return
	}

	if rows == 0 {
		existing, getErr := s.DB.GetAgentLabel(r.Context(), database.GetAgentLabelParams{
			AgentID: mustUUID(agentID),
			Key:     key,
		})
		if errors.Is(getErr, pgx.ErrNoRows) {
			respondError(w, http.StatusNotFound, "label not found")
			return
		}
		if getErr != nil {
			s.dbError(w, getErr, "handleDeleteAgentLabel")
			return
		}
		if existing.Source == "auto" {
			respondError(w, http.StatusForbidden, "cannot delete auto-sourced labels")
			return
		}
		// Source='user' but delete missed: race condition (concurrent delete)
		respondError(w, http.StatusNotFound, "label not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
