package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	maxOverviewPageSize = 200
	maxSearchLen        = 128
)

// ErrInvalidOverviewParam marks a caller-supplied parameter as invalid (bad
// status value, malformed label), distinguishing client errors from DB
// failures so handlers can map them to 400 vs 500.
var ErrInvalidOverviewParam = errors.New("invalid overview parameter")

// OverviewLabelFilter is one AND-combined key=value label constraint.
type OverviewLabelFilter struct {
	Key   string
	Value string
}

// GetOverviewPageParams describes one page request: filters, sort, pagination,
// and the threshold values that drive classification (so the server
// classifies identically to the frontend). WithCount controls whether the total
// row count is computed - false on routine polls to avoid counting the whole
// filtered set every cycle, true on initial load and filter changes.
type GetOverviewPageParams struct {
	Search string
	OS     string
	Arch   string
	Status string
	Labels []OverviewLabelFilter
	// IDs restricts the candidate set to specific agents, AND-combined with
	// any other filters present. Pre-validated pgtype.UUID, not raw strings;
	// format validation happens in the HTTP handler, so a malformed value
	// never reaches this layer. Used by pickers that need details for a small,
	// known set of agents (e.g. a starred list) without paging through the
	// whole fleet.
	IDs []pgtype.UUID

	SortBy  string
	SortDir string
	Limit   int32
	Offset  int32

	WithCount bool

	CPUWarn, CPUCrit    float64
	MemWarn, MemCrit    float64
	DiskWarn, DiskCrit  float64
	TempWarn, TempCrit  float64
	StaleAfterSeconds   int32
	OfflineAfterSeconds int32
}

// GetOverviewPageResult carries one page of rows plus an optional total. Counted
// reports whether Total is meaningful.
type GetOverviewPageResult struct {
	Rows    []GetOverviewRow
	Total   int64
	Counted bool
}

// overviewSortExprs whitelists selectable sort keys to their SQL expressions,
// evaluated against the classified CTE (so column names are unqualified). A key
// absent here falls back to hostname, so the interpolated token is always safe.
// "status" sorts by health severity via the classified status alias, and "net"
// combines rx+tx because the UI shows them as one column.
//
// Each value must be a SINGLE expression: buildOverviewPageQuery writes the
// direction immediately after it, so a comma-separated pair like "os, platform"
// would apply the direction to platform only.
var overviewSortExprs = map[string]string{
	"hostname":  "hostname",
	"os":        "os",
	"platform":  "platform",
	"arch":      "arch",
	"cpu":       "COALESCE(cpu_usage, 0)",
	"memory":    "COALESCE(ram_percent, 0)",
	"disk":      "COALESCE(disk_max_percent, 0)",
	"temp":      "COALESCE(max_temp, 0)",
	"uptime":    "COALESCE(uptime, 0)",
	"procs":     "COALESCE(process_count, 0)",
	"net":       "(COALESCE(net_rx_bytes,0) + COALESCE(net_tx_bytes,0))",
	"severity":  "(COALESCE(cpu_usage,0) + COALESCE(disk_max_percent,0) + COALESCE(ram_percent,0))",
	"last_seen": "last_seen",
	"status":    "CASE status WHEN 'offline' THEN 0 WHEN 'crit' THEN 1 WHEN 'warn' THEN 2 WHEN 'stale' THEN 3 ELSE 4 END",
}

// overviewColumns is the single source of truth for the projected columns, in
// the exact order scanDest scans them. "commit" is quoted (reserved keyword).
// These are the columns of GetOverviewRow; keep this list, scanDest, and the
// sqlc GetOverviewRow struct aligned (a test pins the count).
const overviewColumns = `id, hostname, os, platform, arch, cpu_cores, last_seen,
	ip_address, version, "commit", binary_hash,
	cpu_usage, load_normalized, ram_percent, swap_percent,
	disk_max_percent, net_rx_bytes, net_tx_bytes, max_temp,
	uptime, process_count, reboot_required, updated_at`

// scanDest returns the scan targets for a GetOverviewRow, in overviewColumns
// order. Must stay aligned with overviewColumns and the sqlc struct.
func scanOverviewDest(r *GetOverviewRow) []any {
	return []any{
		&r.ID, &r.Hostname, &r.Os, &r.Platform, &r.Arch, &r.CpuCores, &r.LastSeen,
		&r.IpAddress, &r.Version, &r.Commit, &r.BinaryHash,
		&r.CpuUsage, &r.LoadNormalized, &r.RamPercent, &r.SwapPercent,
		&r.DiskMaxPercent, &r.NetRxBytes, &r.NetTxBytes, &r.MaxTemp,
		&r.Uptime, &r.ProcessCount, &r.RebootRequired, &r.UpdatedAt,
	}
}

// argAcc accumulates positional args and returns $N placeholders.
type argAcc struct{ args []any }

func (a *argAcc) p(v any) string {
	a.args = append(a.args, v)
	return fmt.Sprintf("$%d", len(a.args))
}

// buildClassifiedCTE writes the "WITH classified AS (...)" prefix: all overview
// columns plus the threshold-parameterized status expression, filtered by the
// non-status conditions (os/arch/search/labels). Uses timestamp comparison
// (last_seen <= NOW() - N seconds) rather than EXTRACT(EPOCH...): the comparison
// form is cleaner and more index-friendly in principle. Note that once the value
// is wrapped in the status CASE and filtered via the outer "status = $" alias,
// the planner can't use a last_seen index directly for that filter; the benefit
// is mainly to the expression itself, not the aliased status filter.
func buildClassifiedCTE(sb *strings.Builder, acc *argAcc, arg GetOverviewPageParams) {
	offlineP := acc.p(arg.OfflineAfterSeconds)
	staleP := acc.p(arg.StaleAfterSeconds)
	cpuCritP, memCritP := acc.p(arg.CPUCrit), acc.p(arg.MemCrit)
	diskCritP, tempCritP := acc.p(arg.DiskCrit), acc.p(arg.TempCrit)
	cpuWarnP, memWarnP := acc.p(arg.CPUWarn), acc.p(arg.MemWarn)
	diskWarnP, tempWarnP := acc.p(arg.DiskWarn), acc.p(arg.TempWarn)

	sb.WriteString("WITH classified AS (\n\tSELECT\n\t\ta.id, a.hostname, a.os, a.platform, a.arch, a.cpu_cores, a.last_seen,\n")
	sb.WriteString("\t\ta.ip_address, a.version, a.\"commit\", a.binary_hash,\n")
	sb.WriteString("\t\tm.cpu_usage, m.load_normalized, m.ram_percent, m.swap_percent,\n")
	sb.WriteString("\t\tm.disk_max_percent, m.net_rx_bytes, m.net_tx_bytes, m.max_temp,\n")
	sb.WriteString("\t\tm.uptime, m.process_count, m.reboot_required, m.updated_at,\n")
	fmt.Fprintf(sb, `		CASE
			WHEN a.last_seen IS NULL THEN 'offline'
			WHEN a.last_seen <= NOW() - (%s::int * INTERVAL '1 second') THEN 'offline'
			WHEN a.last_seen <= NOW() - (%s::int * INTERVAL '1 second') THEN 'stale'
			WHEN COALESCE(m.cpu_usage,0) >= %s OR COALESCE(m.ram_percent,0) >= %s
			  OR COALESCE(m.disk_max_percent,0) >= %s OR COALESCE(m.max_temp,0) >= %s THEN 'crit'
			WHEN COALESCE(m.cpu_usage,0) >= %s OR COALESCE(m.ram_percent,0) >= %s
			  OR COALESCE(m.disk_max_percent,0) >= %s OR COALESCE(m.max_temp,0) >= %s THEN 'warn'
			ELSE 'online'
		END AS status
	FROM agents a
	LEFT JOIN current_metrics m ON a.id = m.agent_id`,
		offlineP, staleP,
		cpuCritP, memCritP, diskCritP, tempCritP,
		cpuWarnP, memWarnP, diskWarnP, tempWarnP,
	)

	conds := buildInnerFilters(acc, arg)
	if len(conds) > 0 {
		sb.WriteString("\n\tWHERE ")
		sb.WriteString(strings.Join(conds, "\n\t  AND "))
	}
	sb.WriteString("\n)")
}

// buildInnerFilters returns the non-status WHERE conditions applied inside the
// CTE (os, arch, search, labels), binding values via acc.
func buildInnerFilters(acc *argAcc, arg GetOverviewPageParams) []string {
	var conds []string
	if arg.OS != "" {
		conds = append(conds, "a.os = "+acc.p(arg.OS))
	}
	if arg.Arch != "" {
		conds = append(conds, "a.arch = "+acc.p(arg.Arch))
	}
	if arg.Search != "" {
		esc := escapeLike(arg.Search)
		conds = append(conds, "a.hostname ILIKE "+acc.p("%"+esc+"%")+" ESCAPE '\\'")
	}
	if len(arg.IDs) > 0 {
		conds = append(conds, "a.id = ANY("+acc.p(arg.IDs)+"::uuid[])")
	}
	for _, lm := range arg.Labels {
		conds = append(conds, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM agent_labels al WHERE al.agent_id = a.id AND al.key = %s AND al.value = %s)",
			acc.p(lm.Key), acc.p(lm.Value),
		))
	}
	return conds
}

// escapeLike escapes LIKE/ILIKE metacharacters to the user's search text is
// matched literally rather than as a pattern.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// validOverviewStatuses is the set of accepted status filter values.
var validOverviewStatuses = map[string]struct{}{
	"online": {}, "warn": {}, "crit": {}, "stale": {}, "offline": {},
}

// validateOverviewParams rejects malformed input at the database layer as a
// defense-in-depth check behind handler validation. An unknown status filter or
// a label filter with an empty key/value is a caller bug, so it errors rather
// than silently ignoring the constraint.
func validateOverviewParams(arg GetOverviewPageParams) error {
	if arg.Status != "" {
		if _, ok := validOverviewStatuses[arg.Status]; !ok {
			return fmt.Errorf("%w: invalid status filter %q", ErrInvalidOverviewParam, arg.Status)
		}
	}
	if len(arg.Search) > maxSearchLen {
		return fmt.Errorf("%w: search too long (max %d)", ErrInvalidOverviewParam, maxSearchLen)
	}
	if len(arg.IDs) > maxOverviewPageSize {
		return fmt.Errorf("%w: too many ids (max %d)", ErrInvalidOverviewParam, maxOverviewPageSize)
	}
	for _, lm := range arg.Labels {
		if lm.Key == "" || lm.Value == "" {
			return fmt.Errorf("%w: label filter requires non-empty key and value", ErrInvalidOverviewParam)
		}
	}
	return nil
}

// normalizeOverviewLimitOffset clamps the page bounds: limit into
// [1, maxOverviewPageSize] (defaulting 0 to 25), offset floored at 0. Shared by
// the query builder and the empty-page fallback so both use identical values.
func normalizeOverviewLimitOffset(limit, offset int32) (int32, int32) {
	if limit <= 0 {
		limit = 25
	}
	if limit > maxOverviewPageSize {
		limit = maxOverviewPageSize
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// buildOverviewPageQuery assembles the page SQL and its args from validated
// params. No DB access, so query generation is unit-testable. Applies the sort
// whitelist (unknown -> hostname), the limit clamp ([1, maxOverviewPageSize]),
// and the offset floor (>= 0).
//
// buildOverviewPageQuery assumes validateOverviewParams has already passed.
func buildOverviewPageQuery(arg GetOverviewPageParams) (string, []any) {
	sortExpr, ok := overviewSortExprs[arg.SortBy]
	if !ok {
		sortExpr = "hostname"
	}
	dir := "ASC"
	if strings.EqualFold(arg.SortDir, "desc") {
		dir = "DESC"
	}
	limit, offset := normalizeOverviewLimitOffset(arg.Limit, arg.Offset)

	acc := &argAcc{}
	var sb strings.Builder

	buildClassifiedCTE(&sb, acc, arg)

	sb.WriteString("\nSELECT ")
	sb.WriteString(overviewColumns)
	if arg.WithCount {
		sb.WriteString(", COUNT(*) OVER() AS total_count")
	}
	sb.WriteString("\nFROM classified")
	if arg.Status != "" {
		sb.WriteString("\nWHERE status = ")
		sb.WriteString(acc.p(arg.Status))
	}
	sb.WriteString("\nORDER BY ")
	sb.WriteString(sortExpr)
	sb.WriteByte(' ')
	sb.WriteString(dir)
	// NULLS LAST so rows with no value for the sorted column stay at the
	// bottom in both directions. Postgres defaults to NULLS FIRST on DESC,
	// which would put never-seen agents at the top of a "most recent first"
	// sort. Most exprs are COALESCE'd already, last_seen is not.
	sb.WriteString(" NULLS LAST")
	sb.WriteString(", hostname ASC")
	sb.WriteString("\nLIMIT ")
	sb.WriteString(acc.p(limit))
	sb.WriteString(" OFFSET ")
	sb.WriteString(acc.p(offset))

	return sb.String(), acc.args
}

// GetOverviewPage runs the paginated, filtered, sorted overview query. Single
// shape: classify all candidates in a CTE, then filter/sort/paginate the outer
// query. COUNT(*) OVER() is emitted only when WithCount is true; a past-the-end
// counted page (zero rows, so no row carries the window count) falls back to a
// standalone COUNT.
func (q *Queries) GetOverviewPage(ctx context.Context, arg GetOverviewPageParams) (GetOverviewPageResult, error) {
	var res GetOverviewPageResult

	if err := validateOverviewParams(arg); err != nil {
		return res, err
	}

	// Normalized offset drives the empty-page fallback, matching the value the
	// builder uses in the query.
	_, offset := normalizeOverviewLimitOffset(arg.Limit, arg.Offset)

	sql, args := buildOverviewPageQuery(arg)

	rows, err := q.db.Query(ctx, sql, args...)
	if err != nil {
		return res, fmt.Errorf("overview page query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r GetOverviewRow
		dest := scanOverviewDest(&r)
		if arg.WithCount {
			var total int64
			dest = append(dest, &total)
			if err := rows.Scan(dest...); err != nil {
				return res, fmt.Errorf("overview page scan: %w", err)
			}
			res.Total = total
			res.Counted = true
		} else {
			if err := rows.Scan(dest...); err != nil {
				return res, fmt.Errorf("overview page scan: %w", err)
			}
		}
		res.Rows = append(res.Rows, r)
	}
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("overview page rows: %w", err)
	}

	// Past-the-end counted page: no row carried the window count. Fall back to a
	// standalone COUNT with the same filters. Rare (only after a filter shrinks
	// the set below the current offset), so the common path stays one query.
	if arg.WithCount && !res.Counted && offset > 0 {
		total, err := q.countOverview(ctx, arg)
		if err != nil {
			return res, err
		}
		res.Total = total
		res.Counted = true
	}

	return res, nil
}

// countOverview runs a standalone COUNT with the same filters (including status),
// used only for the past-the-end counted-page fallback. Reuses the classified
// CTE so the status filter applies identically.
func (q *Queries) countOverview(ctx context.Context, arg GetOverviewPageParams) (int64, error) {
	acc := &argAcc{}
	var sb strings.Builder
	buildClassifiedCTE(&sb, acc, arg)
	sb.WriteString("\nSELECT COUNT(*) FROM classified")
	if arg.Status != "" {
		sb.WriteString("\nWHERE status = ")
		sb.WriteString(acc.p(arg.Status))
	}

	var total int64
	if err := q.db.QueryRow(ctx, sb.String(), acc.args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("overview count: %w", err)
	}
	return total, nil
}
