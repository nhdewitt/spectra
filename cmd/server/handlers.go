package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
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
	hostname := r.URL.Query().Get("hostname")
	if hostname == "" {
		http.Error(w, "Missing hostname", http.StatusBadRequest)
		return
	}

	var reader io.ReadCloser = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "Bad Gzip Body", http.StatusBadRequest)
			return
		}
		reader = gz
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
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
			case "service":
				metric = &protocol.ServiceMetric{}
			case "service_list":
				metric = &protocol.ServiceListMetric{}
			default:
				log.Printf("Warning: Unknown metric type received: %s", env.Type)
				continue
			}

			if err := json.Unmarshal(env.Data, metric); err != nil {
				log.Printf("Error unmarshaling type %s data: %v", env.Type, err)
				continue
			}

			if env.Type == "service" {
				s := metric.(*protocol.ServiceMetric)
				fmt.Printf(" [%s] service: %-20s %s (%s)\n", env.Timestamp.Format("15:04:05"), s.Name, s.Status, s.SubStatus)
			} else if env.Type == "service_list" {
				list := metric.(*protocol.ServiceListMetric)
				log.Printf(" [%s] service_list: Received list of %d services", env.Timestamp.Format("15:04:05"), len(list.Services))
				for _, s := range list.Services {
					fmt.Printf(" [%s] service: %-20s %s (%s)\n", env.Timestamp.Format("15:04:05"), s.Name, s.Status, s.SubStatus)
				}
			} else {
				fmt.Printf(" [%s] %s: %v\n", env.Timestamp.Format("15:04:05"), env.Type, metric)
			}
		}
	}()
}

func handleCommandResult(w http.ResponseWriter, r *http.Request) {
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

	var res protocol.CommandResult
	if err := json.NewDecoder(reader).Decode(&res); err != nil {
		http.Error(w, "Bad JSON", http.StatusBadRequest)
		return
	}

	fmt.Printf("\n>>> RESULT RECEIVED FROM %s (CMD: %s) <<<\n", hostname, res.ID)

	if res.Error != "" {
		fmt.Printf(" [ERROR] Agent failed to execute command: %s\n", res.Error)
		w.WriteHeader(http.StatusOK)
		return
	}

	switch res.Type {
	case protocol.CmdFetchLogs:
		var logs []protocol.LogEntry
		if err := json.Unmarshal(res.Payload, &logs); err != nil {
			log.Printf("Failed to unmarshal logs: %v", err)
			return
		}

		for _, l := range logs {
			fmt.Printf(" [LOG] [%s] %s: %s\n", time.Unix(l.Timestamp, 0).Format(time.TimeOnly), l.Source, l.Message)
		}

	case protocol.CmdDiskUsage:
		var report protocol.DiskUsageTopReport

		if err := json.Unmarshal(res.Payload, &report); err != nil {
			log.Printf(" [DISK] Failed to unmarshal report: %v", err)
			return
		}

		fmt.Printf(" [DISK SCAN] Root: %s | Scanned: %d files (%d ms)\n", report.Root, report.ScannedFiles, report.DurationMs)

		if len(report.TopFiles) > 0 {
			fmt.Println(" --- Top Largest Files ---")
			for _, f := range report.TopFiles {
				fmt.Printf("   %-10s %s\n", formatBytes(f.Size), f.Path)
			}
		}
		if len(report.TopDirs) > 0 {
			fmt.Println(" --- Top Largest Directories ---")
			for _, d := range report.TopDirs {
				fmt.Printf("   %-10s %s\n", formatBytes(d.Size), d.Path)
			}
		}

	case protocol.CmdListMounts:
		var mounts []protocol.MountInfo
		if err := json.Unmarshal(res.Payload, &mounts); err != nil {
			log.Printf("Failed to unmarshal mounts: %v", err)
			return
		}
		fmt.Println(" [DISK] Available Mounts:")
		for _, m := range mounts {
			fmt.Printf("   %s (%s) [%s]\n", m.Mountpoint, m.Device, m.FSType)
		}

	}
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

func handleAdminTriggerDisk(store *AgentStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("hostname")
		path := r.URL.Query().Get("path")

		topN := 20
		if val := r.URL.Query().Get("top_n"); val != "" {
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				topN = n
			}
		}

		req := protocol.DiskUsageRequest{
			Path: path,
			TopN: topN,
		}

		payload, err := json.Marshal(req)
		if err != nil {
			http.Error(w, "JSON Marshal Error", http.StatusInternalServerError)
			return
		}

		cmd := protocol.Command{
			ID:      uuid.NewString(),
			Type:    protocol.CmdDiskUsage,
			Payload: payload,
		}

		if store.QueueCommand(hostname, cmd) {
			target := path
			if target == "" {
				target = "Default Drive"
			}

			log.Printf("Queued Disk Scan (Path: %s, TopN: %d) for %s\n", target, topN, hostname)
			fmt.Fprintf(w, "Disk Scan Queued (TopN: %d)\n", topN)
		} else {
			http.Error(w, "Queue full", http.StatusServiceUnavailable)
		}
	}
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

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
