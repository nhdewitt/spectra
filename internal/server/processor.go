package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// processMetric is the entry point for handling a raw metric envelope
func (s *Server) processMetric(agentID string, env RawEnvelope) {
	metric, err := s.unmarshalMetric(env.Type, env.Data)
	if err != nil {
		s.Logger.Warn("error processing metric", "hostname", env.Hostname, "error", err)
		return
	}

	s.persistMetric(context.Background(), agentID, env.Timestamp, metric)
}

// unmarshalMetric converts raw JSON into a concrete protocol.Metric struct
func (s *Server) unmarshalMetric(typ string, data []byte) (protocol.Metric, error) {
	var metric protocol.Metric

	switch typ {
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
	case "application_list":
		metric = &protocol.ApplicationListMetric{}
	case "container":
		metric = &protocol.ContainerMetric{}
	case "container_list":
		metric = &protocol.ContainerListMetric{}
	case "updates":
		metric = &protocol.UpdateMetric{}
	default:
		return nil, fmt.Errorf("unknown metric type: %s", typ)
	}

	if err := json.Unmarshal(data, metric); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", typ, err)
	}

	return metric, nil
}
