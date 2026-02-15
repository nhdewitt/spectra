package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// processMetric is the entry point for handling a raw metric envelope
func (s *Server) processMetric(env RawEnvelope) {
	metric, err := s.unmarshalMetric(env.Type, env.Data)
	if err != nil {
		log.Printf("Error processing metric from %s: %v", env.Hostname, err)
		return
	}

	s.logMetric(env, metric)
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
	case "update_status":
		metric = &protocol.UpdateMetric{}
	default:
		return nil, fmt.Errorf("unknown metric type: %s", typ)
	}

	if err := json.Unmarshal(data, metric); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", typ, err)
	}

	return metric, nil
}

// logMetric is a placeholder fo DB insertion (print to STDOUT)
func (s *Server) logMetric(env RawEnvelope, metric protocol.Metric) {
	ts := env.Timestamp.Format("15:04:05")

	switch m := metric.(type) {
	case *protocol.ServiceMetric:
		fmt.Printf(" [%s] service: %-20s %s (%s)\n", ts, m.Name, m.Status, m.SubStatus)
	case *protocol.ServiceListMetric:
		log.Printf(" [%s] service_list: Received list of %d services", ts, len(m.Services))
		for _, s := range m.Services {
			fmt.Printf(" [%s] service: %-20s %s (%s)\n", ts, s.Name, s.Status, s.SubStatus)
		}
	case *protocol.ApplicationListMetric:
		log.Printf(" [%s] application_list: Received %d applications from %s", ts, len(m.Applications), env.Hostname)
	case *protocol.ContainerListMetric:
		log.Printf(" [%s] container_list: Received %d containers from %s", ts, len(m.Containers), env.Hostname)
		for _, c := range m.Containers {
			fmt.Println(c)
		}
	case *protocol.UpdateMetric:
		log.Printf(" [%s] update_status: %d pending (%d security), reboot=%v [%s]", ts, m.PendingCount, m.SecurityCount, m.RebootRequired, m.PackageManager)
	default:
		fmt.Printf(" [%s] %s: %v\n", ts, env.Type, metric)
	}
}

func (s *Server) logCommandResult(res protocol.CommandResult) {
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

	case protocol.CmdNetworkDiag:
		var report protocol.NetworkDiagnosticReport
		if err := json.Unmarshal(res.Payload, &report); err != nil {
			log.Printf(" [NET] Failed to unmarshal report: %v", err)
			return
		}

		fmt.Printf(" [NET] Action: %s | Target %s\n", report.Action, report.Target)

		if report.Action == "connect" {
			if len(report.PingResults) == 0 {
				fmt.Printf(" [NET] Connect check to %s returned no data.\n", report.Target)
				return
			}

			res := report.PingResults[0]
			status := "CLOSED/FAILED"
			if res.Success {
				status = "OPEN/CONNECTED"
			}

			fmt.Printf(" [NET] Connect: %s -> %s (%s)\n", report.Target, status, res.RTT.Round(time.Millisecond))
			if !res.Success {
				fmt.Printf("      Error: %s\n", res.Response)
			}

			return
		}

		if len(report.PingResults) > 0 {
			fmt.Println(" --- Ping Results ---")
			fmt.Printf("   %-4s %-20s %-12s %s\n", "SEQ", "PEER", "RTT", "STATUS")
			for _, p := range report.PingResults {
				status := p.Response
				if p.Success {
					status = "OK"
				}
				fmt.Printf("   %-4d %-20s %-12s %s\n", p.Seq, p.Peer, p.RTT.Round(time.Millisecond), status)
			}
			return
		}

		if len(report.Netstat) > 0 {
			fmt.Println(" --- Active Connections ---")
			fmt.Printf("   %-5s %-25s %-25s %-12s %s\n", "PROTO", "LOCAL", "REMOTE", "STATE", "USER/PID")
			for i, n := range report.Netstat {
				if i >= 20 {
					break
				}

				local := net.JoinHostPort(n.LocalAddr, fmt.Sprintf("%d", n.LocalPort))
				remote := net.JoinHostPort(n.RemoteAddr, fmt.Sprintf("%d", n.RemotePort))

				owner := ""
				if n.PID > 0 {
					owner = fmt.Sprintf("PID: %d", n.PID)
				} else if n.User != "" {
					owner = fmt.Sprintf("UID: %s", n.User)
				}

				fmt.Printf("   %-5s %-25s %-25s %-12s %s\n", n.Proto, local, remote, n.State, owner)
			}
			return
		}

		if report.RawOutput != "" {
			fmt.Println(" --- Trace Output ---")
			fmt.Println(report.RawOutput)
			return
		}

		fmt.Println(" [NET] Report contained no data.")
	}
}
