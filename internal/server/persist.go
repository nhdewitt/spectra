package server

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/protocol"
)

func uuidParam(id string) pgtype.UUID {
	var u pgtype.UUID
	u.Scan(id)
	return u
}

// persistMetric writes a metric to a database.
func (s *Server) persistMetric(ctx context.Context, agentID string, ts time.Time, metric protocol.Metric) {
	if s.DB == nil {
		return
	}

	uid := uuidParam(agentID)
	t := pgtype.Timestamptz{Time: ts, Valid: true}

	var err error

	switch m := metric.(type) {
	case *protocol.CPUMetric:
		err = s.DB.InsertCPU(ctx, database.InsertCPUParams{
			Time:       t,
			AgentID:    uid,
			Usage:      pgFloat8(m.Usage),
			CoreUsages: float64SliceToPgArray(m.CoreUsage),
			Load1m:     pgFloat8(m.LoadAvg1),
			Load5m:     pgFloat8(m.LoadAvg5),
			Load15m:    pgFloat8(m.LoadAvg15),
			Iowait:     pgFloat8(m.IOWait),
		})

		var normalized float64
		if cores := len(m.CoreUsage); cores > 0 {
			normalized = m.LoadAvg1 / float64(cores)
		}

		if cacheErr := s.DB.UpsertCurrentCPU(ctx, database.UpsertCurrentCPUParams{
			AgentID:        uid,
			CpuUsage:       pgFloat8(m.Usage),
			LoadNormalized: pgFloat8(normalized),
		}); cacheErr != nil {
			log.Printf("Error updating current_metrics (cpu): %v", cacheErr)
		}

	case *protocol.MemoryMetric:
		err = s.DB.InsertMemory(ctx, database.InsertMemoryParams{
			Time:         t,
			AgentID:      uid,
			RamTotal:     pgInt8(int64(m.Total)),
			RamUsed:      pgInt8(int64(m.Used)),
			RamAvailable: pgInt8(int64(m.Available)),
			RamPercent:   pgFloat8(m.UsedPct),
			SwapTotal:    pgInt8(int64(m.SwapTotal)),
			SwapUsed:     pgInt8(int64(m.SwapUsed)),
			SwapPercent:  pgFloat8(m.SwapPct),
		})

		if cacheErr := s.DB.UpsertCurrentMemory(ctx, database.UpsertCurrentMemoryParams{
			AgentID:     uid,
			RamPercent:  pgFloat8(m.UsedPct),
			SwapPercent: pgFloat8(m.SwapPct),
		}); cacheErr != nil {
			log.Printf("Error updating current_metrics (memory): %v", cacheErr)
		}

	case *protocol.DiskMetric:
		err = s.DB.InsertDisk(ctx, database.InsertDiskParams{
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

		if cacheErr := s.DB.UpsertCurrentDiskMax(ctx, uid); cacheErr != nil {
			log.Printf("Error updating current_metrics (disk): %v", cacheErr)
		}

	case *protocol.DiskIOMetric:
		err = s.DB.InsertDiskIO(ctx, database.InsertDiskIOParams{
			Time:         t,
			AgentID:      uid,
			Device:       pgText(m.Device),
			ReadBytes:    pgInt8(int64(m.ReadBytes)),
			WriteBytes:   pgInt8(int64(m.WriteBytes)),
			ReadOps:      pgInt8(int64(m.ReadOps)),
			WriteOps:     pgInt8(int64(m.WriteOps)),
			ReadLatency:  pgInt8(int64(m.ReadTime)),
			WriteLatency: pgInt8(int64(m.WriteTime)),
			IoInProgress: pgInt8(int64(m.InProgress)),
		})

	case *protocol.NetworkMetric:
		err = s.DB.InsertNetwork(ctx, database.InsertNetworkParams{
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

		if cacheErr := s.DB.UpsertCurrentNetwork(ctx, uid); cacheErr != nil {
			log.Printf("Error updating current_metrics (network): %v", cacheErr)
		}

	case *protocol.TemperatureMetric:
		maxTemp := pgtype.Float8{}
		if m.Max != nil {
			maxTemp = pgFloat8(*m.Max)
		}
		err = s.DB.InsertTemperature(ctx, database.InsertTemperatureParams{
			Time:        t,
			AgentID:     uid,
			Sensor:      pgText(m.Sensor),
			Temperature: pgFloat8(m.Temp),
			MaxTemp:     maxTemp,
		})

		if cacheErr := s.DB.UpsertCurrentTemperature(ctx, uid); cacheErr != nil {
			log.Printf("Error updating current_metrics (temperature): %v", cacheErr)
		}

	case *protocol.SystemMetric:
		err = s.DB.InsertSystem(ctx, database.InsertSystemParams{
			Time:         t,
			AgentID:      uid,
			Uptime:       pgInt8(int64(m.Uptime)),
			ProcessCount: pgInt4(int32(m.Processes)),
			UserCount:    pgInt4(int32(m.Users)),
			BootTime:     pgInt8(int64(m.BootTime)),
		})

		if cacheErr := s.DB.UpsertCurrentSystem(ctx, database.UpsertCurrentSystemParams{
			AgentID:      uid,
			Uptime:       pgInt8(int64(m.Uptime)),
			ProcessCount: pgInt4(int32(m.Processes)),
		}); cacheErr != nil {
			log.Printf("Error updating current_metrics (system): %v", cacheErr)
		}

	case *protocol.WiFiMetric:
		err = s.DB.InsertWifi(ctx, database.InsertWifiParams{
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
		err = s.DB.InsertContainer(ctx, database.InsertContainerParams{
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
		for _, c := range m.Containers {
			s.persistMetric(ctx, agentID, ts, &c)
		}
		return

	case *protocol.ProcessListMetric:
		cutoff := pgtype.Timestamptz{Time: ts.Add(-1 * time.Minute), Valid: true}
		for _, p := range m.Processes {
			if upsertErr := s.DB.UpsertProcess(ctx, database.UpsertProcessParams{
				AgentID:    uid,
				Pid:        int32(p.Pid),
				Name:       pgText(p.Name),
				CpuPercent: pgFloat8(p.CPUPercent),
				MemPercent: pgFloat8(p.MemPercent),
				MemRss:     pgInt8(int64(p.MemRSS)),
				Status:     pgText(string(p.Status)),
				Threads:    pgInt4(int32(p.ThreadsTotal)),
			}); upsertErr != nil {
				log.Printf("Error upserting process %d: %v", p.Pid, upsertErr)
			}
		}
		// Remove processes that weren't in this batch
		err = s.DB.DeleteStaleProcesses(ctx, database.DeleteStaleProcessesParams{
			AgentID:   uid,
			UpdatedAt: cutoff,
		})

	case *protocol.ServiceListMetric:
		for _, svc := range m.Services {
			if upsertErr := s.DB.UpsertService(ctx, database.UpsertServiceParams{
				AgentID:   uid,
				Name:      svc.Name,
				Status:    pgText(svc.Status),
				SubStatus: pgText(svc.SubStatus),
			}); upsertErr != nil {
				log.Printf("Error upserting service %s: %v", svc.Name, upsertErr)
			}
		}
		return

	case *protocol.ApplicationListMetric:
		for _, app := range m.Applications {
			if upsertErr := s.DB.UpsertApplication(ctx, database.UpsertApplicationParams{
				AgentID: uid,
				Name:    app.Name,
				Version: pgText(app.Version),
			}); upsertErr != nil {
				log.Printf("Error upserting application %s: %v", app.Name, upsertErr)
			}
		}
		return

	case *protocol.ClockMetric:
		err = s.DB.InsertPi(ctx, database.InsertPiParams{
			Time:       t,
			AgentID:    uid,
			MetricType: "clock",
			ArmFreqHz:  pgInt8(int64(m.ArmFreq)),
			CoreFreqHz: pgInt8(int64(m.CoreFreq)),
			GpuFreqHz:  pgInt8(int64(m.GPUFreq)),
		})

	case *protocol.VoltageMetric:
		err = s.DB.InsertPi(ctx, database.InsertPiParams{
			Time:        t,
			AgentID:     uid,
			MetricType:  "voltage",
			CoreVolts:   pgFloat8(m.Core),
			SdramCVolts: pgFloat8(m.SDRamC),
			SdramIVolts: pgFloat8(m.SDRamI),
			SdramPVolts: pgFloat8(m.SDRamP),
		})

	case *protocol.ThrottleMetric:
		err = s.DB.InsertPi(ctx, database.InsertPiParams{
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
		err = s.DB.InsertPi(ctx, database.InsertPiParams{
			Time:        t,
			AgentID:     uid,
			MetricType:  "gpu",
			GpuMemTotal: pgInt8(int64(m.MemoryTotal)),
			GpuMemUsed:  pgInt8(int64(m.MemoryUsed)),
		})

	case *protocol.UpdateMetric:
		err = s.DB.UpsertUpdates(ctx, database.UpsertUpdatesParams{
			AgentID:        uid,
			PendingCount:   int32(m.PendingCount),
			SecurityCount:  int32(m.SecurityCount),
			RebootRequired: m.RebootRequired,
			PackageManager: pgText(m.PackageManager),
		})

		if cacheErr := s.DB.UpsertCurrentReboot(ctx, database.UpsertCurrentRebootParams{
			AgentID:        uid,
			RebootRequired: m.RebootRequired,
		}); cacheErr != nil {
			log.Printf("Error updating current_metrics (update): %v", cacheErr)
		}

	default:
		// skip silently
		return
	}

	if err != nil {
		log.Printf("Error persisting %s metric: %v", metric.MetricType(), err)
	}
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
