package database

import (
	"context"
	"fmt"
)

// GetOverviewStatsParams carries the threshold values driving classification, so
// the counts match the frontend and the page query exactly.
type GetOverviewStatsParams struct {
	CPUWarn, CPUCrit    float64
	MemWarn, MemCrit    float64
	DiskWarn, DiskCrit  float64
	TempWarn, TempCrit  float64
	StaleAfterSeconds   int32
	OfflineAfterSeconds int32
}

// OverviewStats holds the fleet-wide status counts for the StatBar.
type OverviewStats struct {
	Total   int64
	Online  int64
	Warn    int64
	Crit    int64
	Stale   int64
	Offline int64
	Reboot  int64
}

// GetOverviewStats computes the fleet-wide status distribution in one pass. It
// classifies every agent with the same CASE as the page query, then aggregates
// counts via FILTER clauses. reboot counts agents flagged reboot_required
// regardless of status.
func (q *Queries) GetOverviewStats(ctx context.Context, arg GetOverviewStatsParams) (OverviewStats, error) {
	var st OverviewStats

	const sql = `
WITH classified AS (
	SELECT
		CASE
			WHEN a.last_seen IS NULL THEN 'offline'
			WHEN a.last_seen <= NOW() - ($9::int * INTERVAL '1 second') THEN 'offline'
			WHEN a.last_seen <= NOW() - ($10::int * INTERVAL '1 second') THEN 'stale'
			WHEN COALESCE(m.cpu_usage,0) >= $1 OR COALESCE(m.ram_percent,0) >= $2
			  OR COALESCE(m.disk_max_percent,0) >= $3 OR COALESCE(m.max_temp,0) >= $4 THEN 'crit'
			WHEN COALESCE(m.cpu_usage,0) >= $5 OR COALESCE(m.ram_percent,0) >= $6
			  OR COALESCE(m.disk_max_percent,0) >= $7 OR COALESCE(m.max_temp,0) >= $8 THEN 'warn'
			ELSE 'online'
		END AS status,
		COALESCE(m.reboot_required, false) AS reboot_required
	FROM agents a
	LEFT JOIN current_metrics m ON a.id = m.agent_id
)
SELECT
	COUNT(*) AS total,
	COUNT(*) FILTER (WHERE status = 'online')	AS online,
	COUNT(*) FILTER (WHERE status = 'warn')		AS warn,
	COUNT(*) FILTER (WHERE status = 'crit')		AS crit,
	COUNT(*) FILTER (WHERE status = 'stale')	AS stale,
	COUNT(*) FILTER (WHERE status = 'offline')	AS offline,
	COUNT(*) FILTER (WHERE reboot_required)		AS reboot
FROM classified`

	err := q.db.QueryRow(ctx, sql,
		arg.CPUCrit, arg.MemCrit, arg.DiskCrit, arg.TempCrit,
		arg.CPUWarn, arg.MemWarn, arg.DiskWarn, arg.TempWarn,
		arg.OfflineAfterSeconds, arg.StaleAfterSeconds,
	).Scan(
		&st.Total, &st.Online, &st.Warn, &st.Crit, &st.Stale, &st.Offline, &st.Reboot,
	)
	if err != nil {
		return st, fmt.Errorf("overview stats: %w", err)
	}
	return st, nil
}
