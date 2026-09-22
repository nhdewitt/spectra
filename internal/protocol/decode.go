package protocol

import (
	"encoding/json"
	"fmt"
)

// UnmarshalMetric converts raw JSON into the concerete Metric named by typ.
//
// Lives here rather than in the server so the agent can decode its own
// spooled envelopes back off disk.
func UnmarshalMetric(typ string, data []byte) (Metric, error) {
	var metric Metric

	switch typ {
	case "cpu":
		metric = &CPUMetric{}
	case "memory":
		metric = &MemoryMetric{}
	case "disk":
		metric = &DiskMetric{}
	case "disk_io":
		metric = &DiskIOMetric{}
	case "network":
		metric = &NetworkMetric{}
	case "wifi":
		metric = &WiFiMetric{}
	case "clock":
		metric = &ClockMetric{}
	case "voltage":
		metric = &VoltageMetric{}
	case "throttle":
		metric = &ThrottleMetric{}
	case "gpu":
		metric = &GPUMetric{}
	case "system":
		metric = &SystemMetric{}
	case "process":
		metric = &ProcessMetric{}
	case "process_list":
		metric = &ProcessListMetric{}
	case "temperature":
		metric = &TemperatureMetric{}
	case "service":
		metric = &ServiceMetric{}
	case "service_list":
		metric = &ServiceListMetric{}
	case "application_list":
		metric = &ApplicationListMetric{}
	case "container":
		metric = &ContainerMetric{}
	case "container_list":
		metric = &ContainerListMetric{}
	case "updates":
		metric = &UpdateMetric{}
	default:
		return nil, fmt.Errorf("unknown metric type: %s", typ)
	}

	if err := json.Unmarshal(data, metric); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", typ, err)
	}

	return metric, nil
}
