package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
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
