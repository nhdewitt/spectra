package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

type RawEnvelope struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Hostname  string          `json:"hostname"`
	Data      json.RawMessage `json:"data"`
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
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

func main() {
	http.HandleFunc("/metrics", metricsHandler)
	const listenAddr = "0.0.0.0:8080"
	log.Printf("Starting test server on %s", listenAddr)

	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return nil
		},
	}

	listener, err := lc.Listen(context.Background(), "tcp4", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", listenAddr, err)
	}
	defer listener.Close()

	if err := http.Serve(listener, nil); err != nil {
		log.Fatal(err)
	}
}
