package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/nhdewitt/raspimon/metrics"
)

type RawEnvelope struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Hostname  string          `json:"hostname"`
	Data      json.RawMessage `json:"data"`
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	var rawEnvelopes []RawEnvelope

	err := json.NewDecoder(r.Body).Decode(&rawEnvelopes)
	if err != nil {
		log.Printf("Error decoding batch: %v", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
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
		default:
			log.Printf("Warning: Unknown metric type received: %s", env.Type)
			continue
		}

		if err := json.Unmarshal(env.Data, metric); err != nil {
			log.Printf("Error unmarshaling type %s data: %v", env.Type, err)
			continue
		}

		fmt.Printf(" [%s] %s: %v\n", env.Timestamp.Format("15:04:05"), env.Type, env.Data)
	}

	w.WriteHeader(http.StatusAccepted)
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
