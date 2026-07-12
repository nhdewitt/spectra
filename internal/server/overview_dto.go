package server

import (
	"time"

	"github.com/nhdewitt/spectra/internal/database"
)

// overviewRowToDTO converts a database.GetOverviewRow into the agentOverview DTO
// sent to the dashboard,. unwrapping pgtype nullables into plain values/pointers.
// Shared by handleOverview and handleOverviewPage.
func (s *Server) overviewRowToDTO(row database.GetOverviewRow) agentOverview {
	a := agentOverview{
		Hostname: row.Hostname,
		OS:       row.Os.String,
		CPUCores: row.CpuCores.Int32,
	}

	if row.ID.Valid {
		a.ID = formatUUID(row.ID)
	}
	if row.Arch.Valid {
		a.Arch = row.Arch.String
	}
	if row.Platform.Valid {
		a.Platform = row.Platform.String
	}
	if row.LastSeen.Valid {
		a.LastSeen = row.LastSeen.Time.Format(time.RFC3339)
	}
	if row.UpdatedAt.Valid {
		ts := row.UpdatedAt.Time.Format(time.RFC3339)
		a.MetricsUpdatedAt = &ts
	}
	if row.CpuUsage.Valid {
		a.CPUUsage = &row.CpuUsage.Float64
	}
	if row.LoadNormalized.Valid {
		a.LoadNormalized = &row.LoadNormalized.Float64
	}
	if row.RamPercent.Valid {
		a.RAMPercent = &row.RamPercent.Float64
	}
	if row.SwapPercent.Valid {
		a.SwapPercent = &row.SwapPercent.Float64
	}
	if row.DiskMaxPercent.Valid {
		a.DiskMaxPercent = &row.DiskMaxPercent.Float64
	}
	if row.NetRxBytes.Valid {
		a.NetRxBytes = &row.NetRxBytes.Int64
	}
	if row.NetTxBytes.Valid {
		a.NetTxBytes = &row.NetTxBytes.Int64
	}
	if row.MaxTemp.Valid {
		a.MaxTemp = &row.MaxTemp.Float64
	}
	if row.Uptime.Valid {
		a.Uptime = &row.Uptime.Int64
	}
	if row.ProcessCount.Valid {
		a.ProcessCount = &row.ProcessCount.Int32
	}
	if row.RebootRequired.Valid {
		a.RebootRequired = row.RebootRequired.Bool
	}
	a.Version = row.Version
	a.Commit = row.Commit
	a.BinaryHash = row.BinaryHash

	if s.Releases != nil && a.BinaryHash != "" {
		expected := s.Releases.expectedHash(a.OS, a.Arch)
		a.UpdateAvailable = expected != "" && expected != a.BinaryHash
	}

	return a
}
