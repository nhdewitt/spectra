package server

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

const (
	defaultOverviewPageSize = 25
	maxOverviewPageSizeReq  = 200
	// Keeps (page-1)*size within int32 at the largest page size.
	maxOverviewPage = math.MaxInt32 / maxOverviewPageSizeReq
)

// overviewPage is the paginated response envelope.
type overviewPage struct {
	Agents     []agentOverview `json:"agents"`
	Page       int32           `json:"page"`
	Size       int32           `json:"size"`
	Total      *int64          `json:"total,omitempty"`
	TotalPages *int32          `json:"total_pages,omitempty"`
}

// handleOverviewPage returns one filtered/sorted/paginated page of agents.
//
// Query params:
//
//	page, size			pagination (1-based page; size clamped to [1,200])
//	sort, order			sort key + asc|desc
//	status, os, arch		equality filters
//	search				hostname substring (LIKE-escaped)
//	label				repeatable, "key:value", AND-combined
//	id				repeatable UUID, AND-combined with other filters
//									restricts to specific agents (e.g. a starred list);
//									malformed values reject the whole request with 400
//									rather than being skipped
//	count				"true" to include total/total_pages; omit on routine
//									polls to skip counting
func (s *Server) handleOverviewPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page := parsePositiveInt32(q.Get("page"), 1, maxOverviewPage)
	size := parsePositiveInt32(q.Get("size"), defaultOverviewPageSize, maxOverviewPageSizeReq)
	if size > maxOverviewPageSizeReq {
		size = maxOverviewPageSizeReq
	}

	tv, err := s.getThresholds(r.Context())
	if err != nil {
		s.dbError(w, err, "handleOverviewPage")
		return
	}

	ids, err := parseIDFilters(q["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	params := database.GetOverviewPageParams{
		Search:    q.Get("search"),
		OS:        emptyIfAll(q.Get("os")),
		Arch:      emptyIfAll(q.Get("arch")),
		Status:    emptyIfAll(q.Get("status")),
		Labels:    parseLabelFilters(q["label"]),
		IDs:       ids,
		SortBy:    q.Get("sort"),
		SortDir:   q.Get("order"),
		Limit:     size,
		Offset:    (page - 1) * size,
		WithCount: q.Get("count") == "true",

		CPUWarn:             tv.CPUWarn,
		CPUCrit:             tv.CPUCrit,
		MemWarn:             tv.MemWarn,
		MemCrit:             tv.MemCrit,
		DiskWarn:            tv.DiskWarn,
		DiskCrit:            tv.DiskCrit,
		TempWarn:            tv.TempWarn,
		TempCrit:            tv.TempCrit,
		StaleAfterSeconds:   tv.StaleSeconds,
		OfflineAfterSeconds: tv.OfflineSeconds,
	}

	res, err := s.DB.GetOverviewPage(r.Context(), params)
	if err != nil {
		if errors.Is(err, database.ErrInvalidOverviewParam) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.dbError(w, err, "handleOverviewPage")
		return
	}

	agents := make([]agentOverview, 0, len(res.Rows))
	for _, row := range res.Rows {
		agents = append(agents, s.overviewRowToDTO(row))
	}

	resp := overviewPage{
		Agents: agents,
		Page:   page,
		Size:   size,
	}
	if res.Counted {
		total := res.Total
		resp.Total = &total
		tp := int32((res.Total + int64(size) - 1) / int64(size))
		resp.TotalPages = &tp
	}

	respondJSON(w, http.StatusOK, resp)
}

// parsePositiveInt32 parses a positive int32 query param, returning def on
// missing, invalid, non-positive, or out-of-range input and clamping to limit.
func parsePositiveInt32(val string, def, limit int32) int32 {
	n, err := strconv.ParseInt(val, 10, 32)
	if err != nil || n <= 0 {
		return def
	}
	return min(int32(n), limit)
}

// emptyIfAll normalizes the sentinal "all" and "" to "" so the query treats it
// as no filter. The frontend sends "all" for unset dropdowns.
func emptyIfAll(v string) string {
	if v == "all" {
		return ""
	}
	return v
}

// parseIDFilters converts repeated "id" params into pre-validated
// pgtype.UUID, restricting the candidate set to specific agents. Unlike
// parseLabelFilters, a malformed value here is an error, not a silent skip;
// this filter is used for exact, targeted lookups (e.g. resolving a starred
// list) where dropping a bad ID would silently return the wrong set rather
// than surface a caller bug. Empty strings are ignored.
func parseIDFilters(raw []string) ([]pgtype.UUID, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make([]pgtype.UUID, 0, len(raw))
	for _, id := range raw {
		if id == "" {
			continue
		}
		if !uuidRegex.MatchString(id) {
			return nil, fmt.Errorf("invalid id filter %q", id)
		}
		out = append(out, mustUUID(id))
	}
	return out, nil
}

// praseLabelFilters converts repeated "key:value" label params into filters.
// Malformed entries are skipped here; the DB layer also validates non-empty key/value.
func parseLabelFilters(raw []string) []database.OverviewLabelFilter {
	if len(raw) == 0 {
		return nil
	}
	out := make([]database.OverviewLabelFilter, 0, len(raw))
	for _, lf := range raw {
		key, val, ok := strings.Cut(lf, ":")
		if !ok || key == "" || val == "" {
			continue
		}
		out = append(out, database.OverviewLabelFilter{Key: key, Value: val})
	}
	return out
}
