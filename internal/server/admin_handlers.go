package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/nhdewitt/spectra/internal/protocol"
	"github.com/nhdewitt/spectra/internal/version"
)

func (s *Server) handleAdminTriggerLogs(w http.ResponseWriter, r *http.Request) {
	agentID, ok := s.getTargetAgent(w, r)
	if !ok {
		return
	}

	level := protocol.LogLevel(r.URL.Query().Get("level"))
	if !isValidLogLevel(level) {
		level = protocol.LevelWarning
	}

	req := protocol.LogRequest{MinLevel: level}
	payload, err := json.Marshal(req)
	if err != nil {
		s.Logger.Error("json marshaling failed", "error", err, "handler", "handleAdminTriggerLogs")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.queueHelper(w, agentID, protocol.CmdFetchLogs, payload, "Queued FetchLogs")
}

func isValidLogLevel(l protocol.LogLevel) bool {
	switch l {
	case protocol.LevelDebug, protocol.LevelInfo, protocol.LevelNotice, protocol.LevelWarning,
		protocol.LevelError, protocol.LevelCritical, protocol.LevelAlert, protocol.LevelEmergency:
		return true
	}
	return false
}

func (s *Server) handleAdminTriggerDisk(w http.ResponseWriter, r *http.Request) {
	agentID, ok := s.getTargetAgent(w, r)
	if !ok {
		return
	}

	path := r.URL.Query().Get("path")
	topN := 20
	if val := r.URL.Query().Get("top_n"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			topN = n
		}
	}

	req := protocol.DiskUsageRequest{Path: path, TopN: topN}
	payload, err := json.Marshal(req)
	if err != nil {
		s.Logger.Error("json marshaling failed", "error", err, "handler", "handleAdminTriggerDisk")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.queueHelper(w, agentID, protocol.CmdDiskUsage, payload, fmt.Sprintf("Queued Disk Scan (Top %d)", topN))
}

func (s *Server) handleAdminTriggerNetwork(w http.ResponseWriter, r *http.Request) {
	agentID, ok := s.getTargetAgent(w, r)
	if !ok {
		return
	}

	action := r.URL.Query().Get("action")
	target := r.URL.Query().Get("target")

	if action == "" {
		http.Error(w, "Action required", http.StatusBadRequest)
		return
	}

	req := protocol.NetworkRequest{Action: action, Target: target}
	payload, err := json.Marshal(req)
	if err != nil {
		s.Logger.Error("json marshaling failed", "error", err, "handler", "handleAdminTriggerNetwork")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	s.queueHelper(w, agentID, protocol.CmdNetworkDiag, payload, fmt.Sprintf("Queued Network Diag: %s", action))
}

func (s *Server) handleGenerateToken(w http.ResponseWriter, r *http.Request) {
	token := s.Tokens.Generate(24 * time.Hour)
	s.Logger.Info("registration token generated", "expires_in", "24h")

	respondJSON(w, http.StatusCreated, map[string]string{
		"token": token,
	})
}

func (s *Server) handlePushUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AgentIDs []string `json:"agent_ids"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.AgentIDs) == 0 {
		http.Error(w, "agent_ids required", http.StatusBadRequest)
		return
	}

	if s.Releases == nil {
		http.Error(w, "no releases directory configured", http.StatusServiceUnavailable)
		return
	}

	serverURL := s.Config.ExternalURL
	if serverURL == "" {
		scheme := "https"
		if r.TLS == nil {
			scheme = "http"
		}
		serverURL = fmt.Sprintf("%s://%s", scheme, r.Host)
	}

	ctx := r.Context()
	queued, skipped, failed := 0, 0, 0

	for _, id := range req.AgentIDs {
		agent, err := s.DB.GetAgent(ctx, mustUUID(id))
		if err != nil {
			s.Logger.Warn("update: agent not found", "agent_id", id)
			failed++
			continue
		}

		filename := agentBinaryFilename(agent.Os.String, agent.Arch.String)
		if filename == "" {
			s.Logger.Warn("update: unknown platform", "agent_id", id, "os", agent.Os, "arch", agent.Arch)
			skipped++
			continue
		}

		sha256, ok := s.Releases.get(filename)
		if !ok {
			s.Logger.Warn("update: no binary for platform", "agent_id", id, "filename", filename)
			skipped++
			continue
		}

		downloadURL := fmt.Sprintf("%s/api/v1/admin/releases/%s", serverURL, filename)

		payload, _ := json.Marshal(protocol.UpdateAgentRequest{
			Version: version.Version,
			URL:     downloadURL,
			SHA256:  sha256,
		})

		cmd := protocol.Command{
			ID:      uuid.NewString(),
			Type:    protocol.CmdUpdateAgent,
			Payload: payload,
		}
		if err := s.CmdQueue.Send(id, cmd); err != nil {
			s.Logger.Warn("update: queue failed", "agent_id", id, "error", err)
			failed++
		} else {
			queued++
		}
	}

	s.Logger.Info("agent update pushed",
		"ip", clientIP(r),
		"queued", queued,
		"skipped", skipped,
		"failed", failed)

	respondJSON(w, http.StatusOK, map[string]int{
		"queued":  queued,
		"skipped": skipped,
		"failed":  failed,
	})
}

func agentBinaryFilename(goos, arch string) string {
	if goos == "" || arch == "" {
		return ""
	}
	if goos == "windows" {
		return fmt.Sprintf("spectra-agent-%s-%s.exe", goos, arch)
	}
	return fmt.Sprintf("spectra-agent-%s-%s", goos, arch)
}
