package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

// errTestDB is a non-sentinel error for exercising the 500 path (distinct
// from database.ErrInvalidOverviewParam which maps to 400).
var errTestDB = errors.New("test db failure")

// defaultThresholdsRow returns a thresholds row with the default values so
// getThresholds succeeds in handler tests. Returns the pointer type the mock's
// StatusThresholds field holds.
func defaultThresholdsRow() *database.GetStatusThresholdsRow {
	return &database.GetStatusThresholdsRow{
		CpuWarn: 80, CpuCrit: 95,
		MemWarn: 80, MemCrit: 95,
		DiskWarn: 98, DiskCrit: 99,
		TempWarn: 70, TempCrit: 85,
		StaleSeconds: 120, OfflineSeconds: 600,
	}
}

// seedOverviewRow builds a minimal valid GetOverviewRow for handler tests.
func seedOverviewRow(hostname string) database.GetOverviewRow {
	return database.GetOverviewRow{
		ID:       pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		Hostname: hostname,
		Os:       pgtype.Text{String: "linux", Valid: true},
		Version:  "1.0.0",
	}
}

func TestHandleOverviewPage_NoCount(t *testing.T) {
	s, _, _, mock := newTestServer()
	mock.StatusThresholds = defaultThresholdsRow()
	mock.GetOverviewPageResult = database.GetOverviewPageResult{
		Rows:    []database.GetOverviewRow{seedOverviewRow("seed-web-1")},
		Counted: false,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview/page?page=1&size=25", nil)
	rec := httptest.NewRecorder()
	s.handleOverviewPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp overviewPage
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Agents) != 1 {
		t.Errorf("agents = %d, want 1", len(resp.Agents))
	}
	// No count requested -> Total/TotalPages omitted (nil).
	if resp.Total != nil {
		t.Errorf("Total = %v, want nil (no count)", *resp.Total)
	}
	if resp.TotalPages != nil {
		t.Errorf("TotalPages = %v, want nil (no count)", *resp.TotalPages)
	}
}

func TestHandleOverviewPage_WithCount(t *testing.T) {
	s, _, _, mock := newTestServer()
	mock.StatusThresholds = defaultThresholdsRow()
	mock.GetOverviewPageResult = database.GetOverviewPageResult{
		Rows:    []database.GetOverviewRow{seedOverviewRow("seed-web-1")},
		Total:   51,
		Counted: true,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview/page?page=1&size=25&count=true", nil)
	rec := httptest.NewRecorder()
	s.handleOverviewPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp overviewPage
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total == nil || *resp.Total != 51 {
		t.Errorf("Total = %v, want 51", resp.Total)
	}
	// 51 rows / 25 per page = ceil(2.04) = 3 pages.
	if resp.TotalPages == nil || *resp.TotalPages != 3 {
		t.Errorf("TotalPages = %v, want 3", resp.TotalPages)
	}
}

func TestHandleOverviewPage_TotalPagesCeil(t *testing.T) {
	cases := []struct {
		total int64
		size  int
		want  int32
	}{
		{0, 25, 0},
		{1, 25, 1},
		{25, 25, 1},
		{26, 25, 2},
		{50, 25, 2},
		{51, 25, 3},
	}
	for _, c := range cases {
		s, _, _, mock := newTestServer()
		mock.StatusThresholds = defaultThresholdsRow()
		mock.GetOverviewPageResult = database.GetOverviewPageResult{
			Rows:    []database.GetOverviewRow{},
			Total:   c.total,
			Counted: true,
		}
		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/overview/page?count=true&size="+strconv.Itoa(c.size), nil)
		rec := httptest.NewRecorder()
		s.handleOverviewPage(rec, req)

		var resp overviewPage
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp.TotalPages == nil || *resp.TotalPages != c.want {
			t.Errorf("total=%d size=%d: TotalPages = %v, want %d", c.total, c.size, resp.TotalPages, c.want)
		}
	}
}

func TestHandleOverviewPage_InvalidStatusIs400(t *testing.T) {
	s, _, _, mock := newTestServer()
	mock.StatusThresholds = defaultThresholdsRow()
	mock.GetOverviewPageErr = database.ErrInvalidOverviewParam

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview/page?status=melting", nil)
	rec := httptest.NewRecorder()
	s.handleOverviewPage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid param", rec.Code)
	}
}

func TestHandleOverviewPage_DBErrorIs500(t *testing.T) {
	s, _, _, mock := newTestServer()
	mock.StatusThresholds = defaultThresholdsRow()
	mock.GetOverviewPageErr = errTestDB // any non-sentinel error

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview/page", nil)
	rec := httptest.NewRecorder()
	s.handleOverviewPage(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for DB error", rec.Code)
	}
}
