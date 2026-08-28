package server

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/protocol"
)

// mustUUID converts a validated UUID string to pgtype.UUID.
// It panics if the string is not a valid UUID.
func mustUUID(id string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(id); err != nil {
		panic("mustUUID: invalid UUID after validation")
	}
	return u
}

// persistMetric writes the durable part of a metric through tx.
//
// Only writes whose failure must fail the whole batch belong here; the current_metrics
// refresh is handled separately by refreshCurrent, after the transaction commits. Splitting
// them keeps a broken dashboard cache from forcing replay of history that stored correctly.
//
// The returned error is what makes 202 on /agent/metrics honest. The agent discards a batch
// on any 2xx, so a write failure that is only logged here is data the fleet can never send again.
func (s *Server) persistMetric(ctx context.Context, tx MetricWriter, agentID string, ts time.Time, metric protocol.Metric) error {
	if tx == nil {
		return nil
	}

	uid := mustUUID(agentID)
	t := pgtype.Timestamptz{Time: ts, Valid: true}

	var err error

	switch m := metric.(type) {
	case *protocol.CPUMetric:
		err = tx.InsertCPU(ctx, database.InsertCPUParams{
			Time:       t,
			AgentID:    uid,
			Usage:      pgFloat8(m.Usage),
			CoreUsages: float64SliceToPgArray(m.CoreUsage),
			Load1m:     pgFloat8(m.LoadAvg1),
			Load5m:     pgFloat8(m.LoadAvg5),
			Load15m:    pgFloat8(m.LoadAvg15),
			Iowait:     pgFloat8(m.IOWait),
		})

	case *protocol.MemoryMetric:
		err = tx.InsertMemory(ctx, database.InsertMemoryParams{
			Time:         t,
			AgentID:      uid,
			RamTotal:     pgInt8(int64(m.Total)),
			RamUsed:      pgInt8(int64(m.Used)),
			RamAvailable: pgInt8(int64(m.Available)),
			RamPercent:   pgFloat8(m.UsedPct),
			SwapTotal:    pgInt8(int64(m.SwapTotal)),
			SwapUsed:     pgInt8(int64(m.SwapUsed)),
			SwapPercent:  pgFloat8(m.SwapPct),
			SwapInPages:  pgFloat8Ptr(m.SwapIn),
			SwapOutPages: pgFloat8Ptr(m.SwapOut),
		})

	case *protocol.DiskMetric:
		err = tx.InsertDisk(ctx, database.InsertDiskParams{
			Time:          t,
			AgentID:       uid,
			Device:        pgText(m.Device),
			Mountpoint:    pgText(m.Mountpoint),
			Filesystem:    pgText(m.Filesystem),
			DiskType:      pgText(m.Type),
			TotalBytes:    pgInt8(int64(m.Total)),
			UsedBytes:     pgInt8(int64(m.Used)),
			FreeBytes:     pgInt8(int64(m.Available)),
			UsedPercent:   pgFloat8(m.UsedPct),
			InodesTotal:   pgInt8(int64(m.InodesTotal)),
			InodesUsed:    pgInt8(int64(m.InodesUsed)),
			InodesPercent: pgFloat8(m.InodesPct),
		})

	case *protocol.DiskIOMetric:
		err = tx.InsertDiskIO(ctx, database.InsertDiskIOParams{
			Time:       t,
			AgentID:    uid,
			Device:     pgText(m.Device),
			ReadBytes:  pgInt8(int64(m.ReadBytes)),
			WriteBytes: pgInt8(int64(m.WriteBytes)),
			ReadOps:    pgInt8(int64(m.ReadOps)),
			WriteOps:   pgInt8(int64(m.WriteOps)),

			// Legacy columns: device service time, mislabeled as latency since
			// migration 002. Still written so older agents keep producing rows
			// the old charts can read.
			ReadLatency:  pgInt8(int64(m.ReadTime)),
			WriteLatency: pgInt8(int64(m.WriteTime)),

			// Corrected columns from migration 021. Zero from an older agent that
			// does not compute them.
			ReadLatencyMs:  pgFloat8Ptr(m.ReadLatency),
			WriteLatencyMs: pgFloat8Ptr(m.WriteLatency),
			ReadBusyPct:    pgFloat8Ptr(m.ReadBusyPct),
			WriteBusyPct:   pgFloat8Ptr(m.WriteBusyPct),

			IoInProgress: pgInt8(int64(m.InProgress)),
		})

	case *protocol.NetworkMetric:
		err = tx.InsertNetwork(ctx, database.InsertNetworkParams{
			Time:      t,
			AgentID:   uid,
			Interface: pgText(m.Interface),
			Mac:       pgText(m.MAC),
			Mtu:       pgInt4(int32(m.MTU)),
			Speed:     pgInt8(int64(m.Speed)),
			RxBytes:   pgInt8(int64(m.RxBytes)),
			RxPackets: pgInt8(int64(m.RxPackets)),
			RxErrors:  pgInt8(int64(m.RxErrors)),
			RxDrops:   pgInt8(int64(m.RxDrops)),
			TxBytes:   pgInt8(int64(m.TxBytes)),
			TxPackets: pgInt8(int64(m.TxPackets)),
			TxErrors:  pgInt8(int64(m.TxErrors)),
			TxDrops:   pgInt8(int64(m.TxDrops)),
		})

	case *protocol.TemperatureMetric:
		maxTemp := pgtype.Float8{}
		if m.Max != nil {
			maxTemp = pgFloat8(*m.Max)
		}
		err = tx.InsertTemperature(ctx, database.InsertTemperatureParams{
			Time:        t,
			AgentID:     uid,
			Sensor:      pgText(m.Sensor),
			Temperature: pgFloat8(m.Temp),
			MaxTemp:     maxTemp,
		})

	case *protocol.SystemMetric:
		err = tx.InsertSystem(ctx, database.InsertSystemParams{
			Time:         t,
			AgentID:      uid,
			Uptime:       pgInt8(int64(m.Uptime)),
			ProcessCount: pgInt4(int32(m.Processes)),
			UserCount:    pgInt4(int32(m.Users)),
			BootTime:     pgInt8(int64(m.BootTime)),
		})

	case *protocol.WiFiMetric:
		err = tx.InsertWifi(ctx, database.InsertWifiParams{
			Time:         t,
			AgentID:      uid,
			Interface:    pgText(m.Interface),
			Ssid:         pgText(m.SSID),
			Bssid:        pgText(""),
			FrequencyMhz: pgInt4(int32(m.Frequency * 1000)),
			SignalDbm:    pgInt4(int32(m.SignalLevel)),
			NoiseDbm:     pgInt4(0),
			BitrateMbps:  pgFloat8(m.BitRate),
		})

	case *protocol.ContainerMetric:
		err = tx.InsertContainer(ctx, database.InsertContainerParams{
			Time:        t,
			AgentID:     uid,
			ContainerID: pgText(m.ID),
			Name:        pgText(m.Name),
			Image:       pgText(m.Image),
			State:       pgText(m.State),
			Source:      pgText(m.Source),
			Kind:        pgText(m.Kind),
			CpuPercent:  pgFloat8(m.CPUPercent),
			CpuCores:    pgInt4(int32(m.CPULimitCores)),
			MemoryBytes: pgInt8(int64(m.MemoryBytes)),
			MemoryLimit: pgInt8(int64(m.MemoryLimit)),
			NetRxBytes:  pgInt8(int64(m.NetRxBytes)),
			NetTxBytes:  pgInt8(int64(m.NetTxBytes)),
		})

	case *protocol.ContainerListMetric:
		for i := range m.Containers {
			if err := s.persistMetric(ctx, tx, agentID, ts, &m.Containers[i]); err != nil {
				return err
			}
		}
		return nil

	case *protocol.ProcessListMetric:
		cutoff := pgtype.Timestamptz{Time: ts.Add(-1 * time.Minute), Valid: true}
		for _, p := range m.Processes {
			if upsertErr := tx.UpsertProcess(ctx, database.UpsertProcessParams{
				AgentID:    uid,
				Pid:        int32(p.Pid),
				Name:       pgText(p.Name),
				CpuPercent: pgFloat8(p.CPUPercent),
				MemPercent: pgFloat8(p.MemPercent),
				MemRss:     pgInt8(int64(p.MemRSS)),
				Status:     pgText(string(p.Status)),
				Threads:    pgInt4(int32(p.ThreadsTotal)),
			}); upsertErr != nil {
				// Stop at the first failure: the rest of the list would fail
				// the same way, and the agent resends the whole batch anyway.
				return fmt.Errorf("upsert process %d: %w", p.Pid, upsertErr)
			}
		}
		// Remove processes that weren't in this batch
		err = tx.DeleteStaleProcesses(ctx, database.DeleteStaleProcessesParams{
			AgentID:   uid,
			UpdatedAt: cutoff,
		})

	case *protocol.ServiceListMetric:
		for _, svc := range m.Services {
			if upsertErr := tx.UpsertService(ctx, database.UpsertServiceParams{
				AgentID:   uid,
				Name:      svc.Name,
				Status:    pgText(svc.Status),
				SubStatus: pgText(svc.SubStatus),
			}); upsertErr != nil {
				return fmt.Errorf("upsert service %q: %w", svc.Name, upsertErr)
			}
		}
		return nil

	case *protocol.ApplicationListMetric:
		for _, app := range m.Applications {
			if upsertErr := tx.UpsertApplication(ctx, database.UpsertApplicationParams{
				AgentID: uid,
				Name:    app.Name,
				Version: pgText(app.Version),
			}); upsertErr != nil {
				return fmt.Errorf("upsert application %q: %w", app.Name, upsertErr)
			}
		}
		return nil

	case *protocol.ClockMetric:
		err = tx.InsertPi(ctx, database.InsertPiParams{
			Time:       t,
			AgentID:    uid,
			MetricType: "clock",
			ArmFreqHz:  pgInt8(int64(m.ArmFreq)),
			CoreFreqHz: pgInt8(int64(m.CoreFreq)),
			GpuFreqHz:  pgInt8(int64(m.GPUFreq)),
		})

	case *protocol.VoltageMetric:
		err = tx.InsertPi(ctx, database.InsertPiParams{
			Time:        t,
			AgentID:     uid,
			MetricType:  "voltage",
			CoreVolts:   pgFloat8(m.Core),
			SdramCVolts: pgFloat8(m.SDRamC),
			SdramIVolts: pgFloat8(m.SDRamI),
			SdramPVolts: pgFloat8(m.SDRamP),
		})

	case *protocol.ThrottleMetric:
		err = tx.InsertPi(ctx, database.InsertPiParams{
			Time:                  t,
			AgentID:               uid,
			MetricType:            "throttle",
			Throttled:             pgBool(m.Throttled),
			UnderVoltage:          pgBool(m.Undervoltage),
			FreqCapped:            pgBool(m.ArmFreqCapped),
			SoftTempLimit:         pgBool(m.SoftTempLimit),
			UndervoltageOccurred:  pgBool(m.UndervoltageOccurred),
			FreqCapOccurred:       pgBool(m.FreqCapOccurred),
			ThrottledOccurred:     pgBool(m.ThrottledOccurred),
			SoftTempLimitOccurred: pgBool(m.SoftTempLimitOccurred),
		})

	case *protocol.GPUMetric:
		err = tx.InsertPi(ctx, database.InsertPiParams{
			Time:        t,
			AgentID:     uid,
			MetricType:  "gpu",
			GpuMemTotal: pgInt8(int64(m.MemoryTotal)),
			GpuMemUsed:  pgInt8(int64(m.MemoryUsed)),
		})

	case *protocol.UpdateMetric:
		err = tx.UpsertUpdates(ctx, database.UpsertUpdatesParams{
			AgentID:        uid,
			PendingCount:   int32(m.PendingCount),
			SecurityCount:  int32(m.SecurityCount),
			RebootRequired: m.RebootRequired,
			PackageManager: pgText(m.PackageManager),
		})

	default:
		// Unknown metric type: nothing to write, and nothing the agent can fix
		// by resending.
		return nil
	}

	if err != nil {
		return fmt.Errorf("persist %s: %w", metric.MetricType(), err)
	}
	return nil
}

func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

func pgInt4(n int32) pgtype.Int4 {
	return pgtype.Int4{Int32: n, Valid: true}
}

func pgInt8(n int64) pgtype.Int8 {
	return pgtype.Int8{Int64: n, Valid: true}
}

func pgFloat8Ptr(f *float64) pgtype.Float8 {
	if f == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *f, Valid: true}
}

func pgFloat8(f float64) pgtype.Float8 {
	return pgtype.Float8{Float64: f, Valid: true}
}

func pgBool(b bool) pgtype.Bool {
	return pgtype.Bool{Bool: b, Valid: true}
}

func float64SliceToPgArray(s []float64) []float64 {
	if s == nil {
		return []float64{}
	}
	return s
}

// refreshCurrent updates the derived current_metrics cache for a metric.
//
// Deliberately outside the batch transaction and deliberately silent on failure.
// These rows are recomputed from whatever the next batch delivers, so a failure
// here costs a few seconds of dashboard staleness, while making them transactional
// would mean a broken cache write forced the agent to replay history that had
// already stored correctly.
//
// Running after the commit rather than alongside the inserts also keeps the cache
// from advertising values that a later rollback erased.
func (s *Server) refreshCurrent(ctx context.Context, agentID string, metric protocol.Metric) {
	if s.DB == nil {
		return
	}

	uid := mustUUID(agentID)

	switch m := metric.(type) {
	case *protocol.CPUMetric:
		var normalized float64
		if cores := len(m.CoreUsage); cores > 0 {
			normalized = m.LoadAvg1 / float64(cores)
		}

		if err := s.DB.UpsertCurrentCPU(ctx, database.UpsertCurrentCPUParams{
			AgentID:        uid,
			CpuUsage:       pgFloat8(m.Usage),
			LoadNormalized: pgFloat8(normalized),
		}); err != nil {
			s.Logger.Warn("error updating current_metrics", "metric", "cpu", "error", err)
		}

	case *protocol.MemoryMetric:
		if err := s.DB.UpsertCurrentMemory(ctx, database.UpsertCurrentMemoryParams{
			AgentID:     uid,
			RamPercent:  pgFloat8(m.UsedPct),
			SwapPercent: pgFloat8(m.SwapPct),
		}); err != nil {
			s.Logger.Warn("error updating current_metrics", "metric", "memory", "error", err)
		}

	case *protocol.DiskMetric:
		if err := s.DB.UpsertCurrentDiskMax(ctx, uid); err != nil {
			s.Logger.Warn("error updating current_metrics", "metric", "disk", "error", err)
		}

	case *protocol.NetworkMetric:
		if err := s.DB.UpsertCurrentNetwork(ctx, uid); err != nil {
			s.Logger.Warn("error updating current_metrics", "metric", "network", "error", err)
		}

	case *protocol.TemperatureMetric:
		if err := s.DB.UpsertCurrentTemperature(ctx, uid); err != nil {
			s.Logger.Warn("error updating current_metrics", "metric", "temperature", "error", err)
		}

	case *protocol.SystemMetric:
		if err := s.DB.UpsertCurrentSystem(ctx, database.UpsertCurrentSystemParams{
			AgentID:      uid,
			Uptime:       pgInt8(int64(m.Uptime)),
			ProcessCount: pgInt4(int32(m.Processes)),
		}); err != nil {
			s.Logger.Warn("error updating current_metrics", "metric", "system", "error", err)
		}

	case *protocol.UpdateMetric:
		if err := s.DB.UpsertCurrentReboot(ctx, database.UpsertCurrentRebootParams{
			AgentID:        uid,
			RebootRequired: m.RebootRequired,
		}); err != nil {
			s.Logger.Warn("error updating current_metrics", "metric", "updates", "error", err)
		}

	case *protocol.ContainerListMetric:
		// Containers have no current_metrics row of their own, but the list
		// wrapper is what arrives, so recurse for symmetry with persistMetric.
		for i := range m.Containers {
			s.refreshCurrent(ctx, agentID, &m.Containers[i])
		}
	}
}

// decodedMetric pairs a decoded metric with its envelope timestamp, so handleMetrics
// can decode once and then walk the batch twice: durable writes inside the transaction,
// cache refresh after it commits.
type decodedMetric struct {
	metric protocol.Metric
	ts     time.Time
}
