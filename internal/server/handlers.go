package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/crypto/bcrypt"
)

// RawEnvelope is used for unmarshalling metrics
type RawEnvelope struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Hostname  string          `json:"hostname"`
	Data      json.RawMessage `json:"data"`
}

// handleAgentRegister accepts the HostInfo payload
func (s *Server) handleAgentRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req protocol.RegisterRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !s.Tokens.Validate(req.Token) {
		http.Error(w, "invalid or expired registration token", http.StatusUnauthorized)
		return
	}

	agentID := uuid.New().String()
	secret, err := generateSecret(32)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	s.Store.Register(agentID, secret, req.Info)

	if s.DB != nil {
		if err := s.DB.RegisterAgent(r.Context(), database.RegisterAgentParams{
			ID:         uuidParam(agentID),
			SecretHash: string(hashedSecret),
			Hostname:   req.Info.Hostname,
			Os:         pgText(req.Info.OS),
			Platform:   pgText(req.Info.Platform),
			Arch:       pgText(req.Info.Arch),
			CpuModel:   pgText(req.Info.CPUModel),
			CpuCores:   pgInt4(int32(req.Info.CPUCores)),
			RamTotal:   pgInt8(int64(req.Info.RAMTotal)),
		}); err != nil {
			log.Printf("Error persisting agent registration: %v", err)
		}
	}

	log.Printf("Registered agent %s (%s, %d cores, %s)", req.Info.Hostname, agentID, req.Info.CPUCores, req.Info.Platform)

	respondJSON(w, http.StatusCreated, protocol.RegisterResponse{
		AgentID: agentID,
		Secret:  secret,
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	agentID := getAgentID(r)

	var rawEnvelopes []RawEnvelope
	if err := decodeJSONBody(r, &rawEnvelopes); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)

	go func() {
		log.Printf("--- Received Batch of %d Metrics from %s ---", len(rawEnvelopes), agentID)

		for _, env := range rawEnvelopes {
			s.processMetric(agentID, env)
		}
	}()
}

func (s *Server) handleAgentCommand(w http.ResponseWriter, r *http.Request) {
	agentID := getAgentID(r)

	cmd, err := s.Store.WaitForCommand(r.Context(), agentID, s.Config.CommandTimeout)
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

	fmt.Printf("\n>>> RESULT RECEIVED FROM %s (CMD: %s) <<<\n", agentID, res.ID)

	if res.Error != "" {
		fmt.Printf(" [ERROR] Agent failed to execute command: %s\n", res.Error)
		w.WriteHeader(http.StatusOK)
		return
	}

	s.logCommandResult(res)
}

func (s *Server) handleAdminTriggerLogs(w http.ResponseWriter, r *http.Request) {
	agentID, ok := s.getTargetAgent(w, r)
	if !ok {
		return
	}

	req := protocol.LogRequest{MinLevel: protocol.LevelError}
	payload, _ := json.Marshal(req)

	s.queueHelper(w, agentID, protocol.CmdFetchLogs, payload, "Queued FetchLogs")
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
	payload, _ := json.Marshal(req)

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
	payload, _ := json.Marshal(req)

	s.queueHelper(w, agentID, protocol.CmdNetworkDiag, payload, fmt.Sprintf("Queued Network Diag: %s", action))
}

func (s *Server) handleGenerateToken(w http.ResponseWriter, r *http.Request) {
	token := s.Tokens.Generate(24 * time.Hour)

	respondJSON(w, http.StatusCreated, map[string]string{
		"token": token,
	})
}
