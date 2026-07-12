package database

import (
	"strings"
	"testing"
)

// baseParams returns params with the ten threshold values set, so arg counting
// in the generated query is deterministic across tests.
func baseParams() GetOverviewPageParams {
	return GetOverviewPageParams{
		SortBy:              "hostname",
		SortDir:             "asc",
		Limit:               25,
		Offset:              0,
		CPUWarn:             80,
		CPUCrit:             95,
		MemWarn:             80,
		MemCrit:             95,
		DiskWarn:            98,
		DiskCrit:            99,
		TempWarn:            70,
		TempCrit:            85,
		StaleAfterSeconds:   120,
		OfflineAfterSeconds: 600,
	}
}

const thresholdArgCount = 10

func TestBuildQuery_NoCount(t *testing.T) {
	sql, args := buildOverviewPageQuery(baseParams())

	if strings.Contains(sql, "COUNT(*) OVER()") {
		t.Error("WithCount=false should not emit COUNT(*) OVER()")
	}

	// args: 10 thresholds + limit + offset
	if len(args) != thresholdArgCount+2 {
		t.Errorf("args = %d, want %d", len(args), thresholdArgCount+2)
	}
}

func TestBuildQuery_WithCount(t *testing.T) {
	p := baseParams()
	p.WithCount = true
	sql, _ := buildOverviewPageQuery(p)

	if !strings.Contains(sql, "COUNT(*) OVER() AS total_count") {
		t.Error("WithCount=true should emit COUNT(*) OVER()")
	}
}

func TestBuildQuery_StatusFilter(t *testing.T) {
	p := baseParams()
	p.Status = "crit"
	sql, args := buildOverviewPageQuery(p)

	if !strings.Contains(sql, "FROM classified\nWHERE status = ") {
		t.Errorf("status filter should add outer WHERE status =; got:\n%s", sql)
	}

	// 10 thresholds + status + limit + offset
	if len(args) != thresholdArgCount+3 {
		t.Errorf("args = %d, want %d", len(args), thresholdArgCount+3)
	}
	if args[thresholdArgCount] != "crit" {
		t.Errorf("status arg = %v, want crit", args[thresholdArgCount])
	}
}

func TestBuildQuery_NegativeOffsetClamped(t *testing.T) {
	p := baseParams()
	p.Offset = -5
	_, args := buildOverviewPageQuery(p)

	if got := args[len(args)-1]; got != int32(0) {
		t.Errorf("offset arg = %v, want 0", got)
	}
}

func TestBuildQuery_OversizedLimitClamped(t *testing.T) {
	p := baseParams()
	p.Limit = 100000
	_, args := buildOverviewPageQuery(p)

	if got := args[len(args)-2]; got != int32(maxOverviewPageSize) {
		t.Errorf("limit arg = %v, want %d", got, maxOverviewPageSize)
	}
}

func TestBuildQuery_ZeroLimitDefaults(t *testing.T) {
	p := baseParams()
	p.Limit = 0
	_, args := buildOverviewPageQuery(p)

	if got := args[len(args)-2]; got != int32(25) {
		t.Errorf("limit arg = %v, want 25 default", got)
	}
}

func TestBuildQuery_UnknownSortFallsBackToHostname(t *testing.T) {
	p := baseParams()
	p.SortBy = "invalid"
	sql, _ := buildOverviewPageQuery(p)

	if !strings.Contains(sql, "ORDER BY hostname ASC") {
		t.Errorf("unknown sort should fall back to hostname; got:\n%s", sql)
	}
}

func TestBuildQuery_DescDirection(t *testing.T) {
	p := baseParams()
	p.SortBy = "cpu"
	p.SortDir = "desc"
	sql, _ := buildOverviewPageQuery(p)

	if !strings.Contains(sql, "ORDER BY COALESCE(cpu_usage, 0) DESC") {
		t.Errorf("SortDir=desc should order cpu DESC; got:\n%s", sql)
	}
}

func TestBuildQuery_InvalidDirectionFallsBackToAsc(t *testing.T) {
	p := baseParams()
	p.SortDir = "sideways"
	sql, _ := buildOverviewPageQuery(p)

	if !strings.Contains(sql, "ORDER BY hostname ASC") {
		t.Errorf("invalid sort dir should fall back to ASC; got:\n%s", sql)
	}
}

// Label filters emit one EXISTS each and bind two args each.
func TestBuildQuery_LabelFilters(t *testing.T) {
	p := baseParams()
	p.Labels = []OverviewLabelFilter{
		{Key: "env", Value: "prod"},
		{Key: "role", Value: "db"},
	}
	sql, args := buildOverviewPageQuery(p)

	if got := strings.Count(sql, "EXISTS (SELECT 1 FROM agent_labels"); got != 2 {
		t.Errorf("EXISTS count = %d, want 2; got:\n%s", got, sql)
	}
	// thresholds + 2labels*2 + limit + offset
	if len(args) != thresholdArgCount+4+2 {
		t.Errorf("args = %d, want %d", len(args), thresholdArgCount+4+2)
	}
}

func TestBuildQuery_StatusSort(t *testing.T) {
	p := baseParams()
	p.SortBy = "status"
	sql, _ := buildOverviewPageQuery(p)

	if !strings.Contains(sql, "ORDER BY CASE status WHEN 'offline' THEN 0") {
		t.Errorf("status sort should use severity CASE; got:\n%s", sql)
	}
}

func TestBuildQuery_SearchFilter(t *testing.T) {
	p := baseParams()
	p.Search = "agent-01"
	sql, args := buildOverviewPageQuery(p)

	if !strings.Contains(sql, "a.hostname ILIKE ") {
		t.Errorf("search should add hostname ILIKE filter; got:\n%s", sql)
	}
	if got := args[thresholdArgCount]; got != "%agent-01%" {
		t.Errorf("search arg = %v, want %%agent-01%%", got)
	}
}

func TestBuildQuery_SearchEscapeMetacharacters(t *testing.T) {
	p := baseParams()
	p.Search = "web_01%"
	_, args := buildOverviewPageQuery(p)

	if got := args[thresholdArgCount]; got != `%web\_01\%%` {
		t.Errorf("search arg = %v, want %%web\\_01\\%%%%", got)
	}
}

func TestBuildQuery_SearchHasEscapeClause(t *testing.T) {
	p := baseParams()
	p.Search = "x"
	sql, _ := buildOverviewPageQuery(p)

	if !strings.Contains(sql, `ILIKE $`) || !strings.Contains(sql, `ESCAPE '\'`) {
		t.Errorf("search should use ILIKE ... ESCAPE '\\'; got:\n%s", sql)
	}
}

func TestBuildQuery_NoSearchNoILIKE(t *testing.T) {
	sql, _ := buildOverviewPageQuery(baseParams())
	if strings.Contains(sql, "ILIKE") {
		t.Error("empty search should not contain an ILIKE clause")
	}
}

// The projected column list, the inner CTE column list, and scanOverviewDest
// must all agree in count. A mismatch scans into the wrong fields silently.
// This pins the projected-column count to the scan-dest length.
func TestColumnAlignment(t *testing.T) {
	projected := countColumns(overviewColumns)
	scanTargets := len(scanOverviewDest(&GetOverviewRow{}))

	if projected != scanTargets {
		t.Errorf("overviewColumns has %d columns but scanOverviewDest has %d targets; they must match", projected, scanTargets)
	}
}

// countColumns counts comma-separated columns in a projection string, ignoring
// the commas inside function calls like COALESCE(...) - overviewColumns has
// none (plain column names only), so a simple comma count is correct here.
func countColumns(cols string) int {
	cols = strings.TrimSpace(cols)
	if cols == "" {
		return 0
	}
	return strings.Count(cols, ",") + 1
}

func TestValidateOverviewParams(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*GetOverviewPageParams)
		wantErr bool
	}{
		{"valid empty status", func(p *GetOverviewPageParams) { p.Status = "" }, false},
		{"valid crit", func(p *GetOverviewPageParams) { p.Status = "crit" }, false},
		{"invalid status", func(p *GetOverviewPageParams) { p.Status = "melting" }, true},
		{"empty label key", func(p *GetOverviewPageParams) {
			p.Labels = []OverviewLabelFilter{{Key: "", Value: "prod"}}
		}, true},
		{"empty label value", func(p *GetOverviewPageParams) {
			p.Labels = []OverviewLabelFilter{{Key: "env", Value: ""}}
		}, true},
		{"valid label", func(p *GetOverviewPageParams) {
			p.Labels = []OverviewLabelFilter{{Key: "env", Value: "prod"}}
		}, false},
		{"search at limit", func(p *GetOverviewPageParams) {
			p.Search = strings.Repeat("a", maxSearchLen)
		}, false},
		{"search over limit", func(p *GetOverviewPageParams) {
			p.Search = strings.Repeat("a", maxSearchLen+1)
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := baseParams()
			tt.mutate(&p)
			err := validateOverviewParams(p)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"plain":   "plain",
		"50%":     `50\%`,
		"a_b":     `a\_b`,
		`back\sl`: `back\\sl`,
		`a\%b`:    `a\\\%b`,
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}
