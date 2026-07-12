package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/nhdewitt/spectra/internal/database"
)

const (
	defaultOverviewPageSize = 25
	maxOverviewPageSizeReq  = 200
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
//	count				"true" to include total/total_pages; omit on routine
//									polls to skip counting
func (s *Server) handleOverviewPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page := parsePositiveInt(q.Get("page"), 1)
	size := parsePositiveInt(q.Get("size"), defaultOverviewPageSize)
	if size > maxOverviewPageSizeReq {
		size = maxOverviewPageSizeReq
	}

	tv, err := s.getThresholds(r.Context())
	if err != nil {
		s.dbError(w, err, "handleOverviewPage")
		return
	}

	params := database.GetOverviewPageParams{
		Search:    q.Get("search"),
		OS:        emptyIfAll(q.Get("os")),
		Arch:      emptyIfAll(q.Get("arch")),
		Status:    emptyIfAll(q.Get("status")),
		Labels:    parseLabelFilters(q["label"]),
		SortBy:    q.Get("sort"),
		SortDir:   q.Get("order"),
		Limit:     int32(size),
		Offset:    int32((page - 1) * size),
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
		Page:   int32(page),
		Size:   int32(size),
	}
	if res.Counted {
		total := res.Total
		resp.Total = &total
		tp := int32((res.Total + int64(size) - 1) / int64(size))
		resp.TotalPages = &tp
	}

	respondJSON(w, http.StatusOK, resp)
}

// parsePositiveInt parses a positive int query param, falling back to def on
// missing/invalid/non-positive input. Mirrors the top_n parsing style.
func parsePositiveInt(val string, def int) int {
	if val == "" {
		return def
	}
	if n, err := strconv.Atoi(val); err != nil && n > 0 {
		return n
	}
	return def
}

// emptyIfAll normalizes the sentinal "all" and "" to "" so the query treats it
// as no filter. The frontend sends "all" for unset dropdowns.
func emptyIfAll(v string) string {
	if v == "all" {
		return ""
	}
	return v
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
