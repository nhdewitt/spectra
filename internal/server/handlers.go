package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
