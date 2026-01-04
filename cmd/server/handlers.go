package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nhdewitt/spectra/internal/protocol"
)

// RawEnvelope handles the generic unmarshalling
type RawEnvelope struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Hostname  string          `json:"hostname"`
	Data      json.RawMessage `json:"data"`
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r.Body); err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)

	go func() {
		var rawEnvelopes []RawEnvelope

		if err := json.Unmarshal(buf.Bytes(), &rawEnvelopes); err != nil {
			log.Printf("ERROR: Asynchronous decoding failure: %v", err)
			return
		}

		log.Printf("--- Received Batch of %d Metrics from %s ---", len(rawEnvelopes), rawEnvelopes[0].Hostname)

		for _, env := range rawEnvelopes {
			var metric protocol.Metric

			switch env.Type {
			case "cpu":
				metric = &protocol.CPUMetric{}
			case "memory":
				metric = &protocol.MemoryMetric{}
			case "disk":
				metric = &protocol.DiskMetric{}
			case "disk_io":
				metric = &protocol.DiskIOMetric{}
			case "network":
				metric = &protocol.NetworkMetric{}
			case "wifi":
				metric = &protocol.WiFiMetric{}
			case "clock":
				metric = &protocol.ClockMetric{}
			case "voltage":
				metric = &protocol.VoltageMetric{}
			case "throttle":
				metric = &protocol.ThrottleMetric{}
			case "gpu":
				metric = &protocol.GPUMetric{}
			case "system":
				metric = &protocol.SystemMetric{}
			case "process":
				metric = &protocol.ProcessMetric{}
			case "process_list":
				metric = &protocol.ProcessListMetric{}
			case "temperature":
				metric = &protocol.TemperatureMetric{}
			default:
				log.Printf("Warning: Unknown metric type received: %s", env.Type)
				continue
			}

			if err := json.Unmarshal(env.Data, metric); err != nil {
				log.Printf("Error unmarshaling type %s data: %v", env.Type, err)
				continue
			}

			fmt.Printf(" [%s] %s: %v\n", env.Timestamp.Format("15:04:05"), env.Type, metric)
		}
	}()
}

func handleAgentCommand(store *AgentStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("hostname")
		if hostname == "" {
			http.Error(w, "Hostname required", http.StatusBadRequest)
			return
		}

		cmd, found := store.WaitForCommand(hostname, 30*time.Second)

		if !found {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cmd)
	}
}

func handleAgentLogs(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("handleAgentLogs hit: method=%s path=%s query=%s encoding=%s\n",
		r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Content-Encoding"))

	hostname := r.URL.Query().Get("hostname")

	var reader io.ReadCloser = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "Decompression failed", http.StatusBadRequest)
			return
		}
		reader = gz
	}
	defer reader.Close()

	var logs []protocol.LogEntry
	if err := json.NewDecoder(reader).Decode(&logs); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	fmt.Printf("\n=== INCOMING LOGS FROM %s ===\n", hostname)
	for _, l := range logs {
		fmt.Printf("[%s] [%s] %s: %s\n", time.Unix(l.Timestamp, 0), l.Level, l.Source, l.Message)
	}
	fmt.Printf("=============================\n")

	w.WriteHeader(http.StatusOK)
}

func handleAdminTriggerLogs(store *AgentStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("hostname")

		req := protocol.LogRequest{
			MinLevel: protocol.LevelError,
		}
		payload, err := json.Marshal(req)
		if err != nil {
			http.Error(w, "Error marshalling JSON", http.StatusInternalServerError)
			return
		}

		cmd := protocol.Command{
			ID:      uuid.NewString(),
			Type:    protocol.CmdFetchLogs,
			Payload: payload,
		}

		if success := store.QueueCommand(hostname, cmd); success {
			fmt.Printf("Queued FetchLogs command for %s\n", hostname)
			w.Write([]byte("Command Queued\n"))
		} else {
			fmt.Printf("ERROR: Queue full for %s.\n", hostname)
			http.Error(w, "Agent queue full", http.StatusServiceUnavailable)
		}
	}
}
