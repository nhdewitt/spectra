package server

import (
	"encoding/json"
	"net/http"

	"github.com/nhdewitt/spectra/internal/database"
)

// handleGetUserConfig returns all config entries for the current user.
//
// GET /api/v1/user/config
func (s *Server) handleGetUserConfig(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	rows, err := s.DB.GetUserConfig(r.Context(), mustUUID(u.ID))
	if err != nil {
		s.dbError(w, err, "handleGetUserConfig")
		return
	}

	config := make(map[string]json.RawMessage, len(rows))
	for _, row := range rows {
		config[row.ConfigKey] = row.ConfigValue
	}

	respondJSON(w, http.StatusOK, config)
}

// handleSetUserConfig sets a config key for the current user.
//
// PUT /api/v1/user/config
func (s *Server) handleSetUserConfig(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
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
	if len(req.Value) == 0 {
		http.Error(w, "value is required", http.StatusBadRequest)
		return
	}

	if err := s.DB.SetUserConfig(r.Context(), database.SetUserConfigParams{
		UserID:      mustUUID(u.ID),
		ConfigKey:   req.Key,
		ConfigValue: req.Value,
	}); err != nil {
		s.dbError(w, err, "handleSetUserConfig")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteUserConfig removes a config key for the current user.
//
// DELETE /api/v1/user/config
func (s *Server) handleDeleteUserConfig(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key parameter is required", http.StatusBadRequest)
		return
	}

	if err := s.DB.DeleteUserConfig(r.Context(), database.DeleteUserConfigParams{
		UserID:    mustUUID(u.ID),
		ConfigKey: key,
	}); err != nil {
		s.dbError(w, err, "handleDeleteUserConfig")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
