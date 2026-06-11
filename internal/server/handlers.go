package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/labels"
	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/version"
)

// RawEnvelope is used for unmarshalling metrics
type RawEnvelope struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Hostname  string          `json:"hostname"`
	Data      json.RawMessage `json:"data"`
}

// generateAgentSecret creates a 32-byte random secret, returned as hex.
func generateAgentSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashAgentSecret returns the raw SHA-256 bytes of a secret string.
func hashAgentSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// handleAgentRegister accepts the HostInfo payload
func (s *Server) handleAgentRegister(w http.ResponseWriter, r *http.Request) {
	var req protocol.RegisterRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !s.Tokens.Validate(req.Token) {
		s.Logger.Warn("invalid registration token", "hostname", req.Info.Hostname, "ip", clientIP(r))
		http.Error(w, "invalid or expired registration token", http.StatusUnauthorized)
		return
	}

	agentID := uuid.New().String()
	secret, err := generateAgentSecret()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if s.DB != nil {
		if err := s.DB.RegisterAgent(r.Context(), database.RegisterAgentParams{
			ID:           mustUUID(agentID),
			SecretSha256: hashAgentSecret(secret),
			SecretHash:   "",
			Hostname:     req.Info.Hostname,
			Os:           pgText(req.Info.OS),
			Platform:     pgText(req.Info.Platform),
			Arch:         pgText(req.Info.Arch),
			CpuModel:     pgText(req.Info.CPUModel),
			CpuCores:     pgInt4(int32(req.Info.CPUCores)),
			RamTotal:     pgInt8(int64(req.Info.RAMTotal)),
			IpAddress:    pgText(clientIP(r)),
			Version:      req.Info.AgentVer,
		}); err != nil {
			s.Logger.Error("database query error", "error", err, "handler", "handleAgentRegister")
			http.Error(w, "registration failed", http.StatusInternalServerError)
			return
		}
	}

	s.Logger.Info("registered agent",
		"hostname", req.Info.Hostname,
		"agent_id", agentID,
		"cpu_cores", req.Info.CPUCores,
		"platform", req.Info.Platform,
	)

	autoInfo := labels.AgentInfo{
		OS:           req.Info.OS,
		Arch:         req.Info.Arch,
		Hardware:     req.Info.Hardware,
		AgentVersion: req.Info.AgentVer,
	}
	if err := s.syncAutoLabelsOnRegister(r.Context(), agentID, autoInfo); err != nil {
		s.Logger.Warn("auto label sync failed on register",
			"agent_id", agentID, "err", err)
	}

	respondJSON(w, http.StatusCreated, protocol.RegisterResponse{
		AgentID: agentID,
		Secret:  secret,
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	agentID := getAgentID(r)

	if v := r.Header.Get("X-Spectra-Agent-Version"); v != "" {
		if err := s.syncAgentVersionLabel(r.Context(), agentID, v); err != nil {
			s.Logger.Warn("agent_version sync failed",
				"agent_id", agentID, "err", err)
		}
	}

	var rawEnvelopes []RawEnvelope
	if err := decodeJSONBody(r, &rawEnvelopes); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if s.DB != nil {
		if err := s.DB.TouchLastSeenIfStale(r.Context(), database.TouchLastSeenIfStaleParams{
			ID:         mustUUID(agentID),
			IpAddress:  pgText(clientIP(r)),
			Version:    r.Header.Get("X-Agent-Version"),
			Commit:     r.Header.Get("X-Agent-Commit"),
			BinaryHash: r.Header.Get("X-Agent-Binary-Hash"),
		}); err != nil {
			s.Logger.Error("database query error", "error", err, "handler", "handleMetrics")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)

	go func() {
		for _, env := range rawEnvelopes {
			select {
			case <-s.done:
				return
			default:
				s.processMetric(agentID, env)
			}
		}
	}()
}

func (s *Server) handleAgentCommand(w http.ResponseWriter, r *http.Request) {
	agentID := getAgentID(r)

	cmd, err := s.CmdQueue.Wait(r.Context(), agentID, s.Config.CommandTimeout)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	respondJSON(w, http.StatusOK, cmd)
}

func (s *Server) handleCommandResult(w http.ResponseWriter, r *http.Request) {
	agentID := getAgentID(r)

	var res protocol.CommandResult
	if err := decodeJSONBody(r, &res); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.Logger.Info("command result received", "agent_id", agentID, "command", res.ID, "type", res.Type)
	s.Commands.Complete(res.ID, res)

	if res.Error != "" {
		s.Logger.Warn("command failed", "command", res.ID, "error", res.Error)
	}

	w.WriteHeader(http.StatusOK)
}

// handleGetCommandResult returns the status/result of a queued command.
//
// GET /api/v1/admin/commands/{id}
func (s *Server) handleGetCommandResult(w http.ResponseWriter, r *http.Request) {
	cmdID := r.PathValue("id")
	if cmdID == "" {
		http.Error(w, "command ID required", http.StatusBadRequest)
		return
	}

	entry, ok := s.Commands.Get(cmdID)
	if !ok {
		http.Error(w, "command not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, entry)
}

// handleVersion returns the version of the binary build.
//
// GET /api/v1/version
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"version": version.Version,
		"commit":  version.Commit,
		"date":    version.Date,
	})
}

// handlePurgeOfflineAgents removes agents not seen in 7+ days.
//
// POST /api/v1/admin/agents/purge
func (s *Server) handlePurgeOfflineAgents(w http.ResponseWriter, r *http.Request) {
	count, err := s.DB.PurgeOfflineAgents(r.Context())
	if err != nil {
		s.dbError(w, err, "handlePurgeOfflineAgents")
		return
	}

	s.Logger.Info("purged offline agents", "count", count)
	respondJSON(w, http.StatusOK, map[string]int64{"purged": count})
}

// handleRevokeAllTokens invalidates all pending registration tokens.
//
// POST /api/v1/admin/tokens/revoke
func (s *Server) handleRevokeAllTokens(w http.ResponseWriter, r *http.Request) {
	s.Tokens.RevokeAll()
	s.Logger.Info("all registration tokens revoked")
	w.WriteHeader(http.StatusNoContent)
}
