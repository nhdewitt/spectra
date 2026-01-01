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

	"github.com/nhdewitt/spectra/metrics"
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

		log.Printf("--- Received Batch of %d Metrics ---", len(rawEnvelopes))

		for _, env := range rawEnvelopes {
			var metric metrics.Metric

			switch env.Type {
			case "cpu":
				metric = &metrics.CPUMetric{}
			case "memory":
				metric = &metrics.MemoryMetric{}
			case "disk":
				metric = &metrics.DiskMetric{}
			case "disk_io":
				metric = &metrics.DiskIOMetric{}
			case "network":
				metric = &metrics.NetworkMetric{}
			case "wifi":
				metric = &metrics.WiFiMetric{}
			case "clock":
				metric = &metrics.ClockMetric{}
			case "voltage":
				metric = &metrics.VoltageMetric{}
			case "throttle":
				metric = &metrics.ThrottleMetric{}
			case "gpu":
				metric = &metrics.GPUMetric{}
			case "system":
				metric = &metrics.SystemMetric{}
			case "process":
				metric = &metrics.ProcessMetric{}
			case "process_list":
				metric = &metrics.ProcessListMetric{}
			case "temperature":
				metric = &metrics.TemperatureMetric{}
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
