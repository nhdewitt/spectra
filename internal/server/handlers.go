package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
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

	var info protocol.HostInfo
	if err := decodeJSONBody(r, &info); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: DB agent registration
	log.Printf("Registered Agent: %s (%s, %d cores, %s RAM)", info.Hostname, info.Platform, info.CPUCores, formatBytes(info.RAMTotal))

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	hostname, ok := getHostname(w, r)
	if !ok {
		return
	}

	var rawEnvelopes []RawEnvelope
	if err := decodeJSONBody(r, &rawEnvelopes); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)

	go func() {
		log.Printf("--- Received Batch of %d Metrics from %s ---", len(rawEnvelopes), hostname)

		for _, env := range rawEnvelopes {
			s.processMetric(env)
		}
	}()
}

func (s *Server) handleAgentCommand(w http.ResponseWriter, r *http.Request) {
	hostname, ok := getHostname(w, r)
	if !ok {
		return
	}

	cmd, found := s.Store.WaitForCommand(hostname, 30*time.Second)
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	respondJSON(w, http.StatusOK, cmd)
}

func (s *Server) handleCommandResult(w http.ResponseWriter, r *http.Request) {
	hostname, ok := getHostname(w, r)
	if !ok {
		return
	}

	var res protocol.CommandResult
	if err := decodeJSONBody(r, &res); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Printf("\n>>> RESULT RECEIVED FROM %s (CMD: %s) <<<\n", hostname, res.ID)

	if res.Error != "" {
		fmt.Printf(" [ERROR] Agent failed to execute command: %s\n", res.Error)
		w.WriteHeader(http.StatusOK)
		return
	}

	s.logCommandResult(res)
}

func (s *Server) handleAdminTriggerLogs(w http.ResponseWriter, r *http.Request) {
	hostname, ok := getHostname(w, r)
	if !ok {
		return
	}

	req := protocol.LogRequest{MinLevel: protocol.LevelError}
	payload, _ := json.Marshal(req)

	s.queueHelper(w, hostname, protocol.CmdFetchLogs, payload, "Queued FetchLogs")
}

func (s *Server) handleAdminTriggerDisk(w http.ResponseWriter, r *http.Request) {
	hostname, ok := getHostname(w, r)
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

	s.queueHelper(w, hostname, protocol.CmdDiskUsage, payload, fmt.Sprintf("Queued Disk Scan (Top %d)", topN))
}

func (s *Server) handleAdminTriggerNetwork(w http.ResponseWriter, r *http.Request) {
	hostname, ok := getHostname(w, r)
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

	s.queueHelper(w, hostname, protocol.CmdNetworkDiag, payload, fmt.Sprintf("Queued Network Diag: %s", action))
}
