package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

const testUUID = "550e8400-e29b-41d4-a716-446655440000"

// --- Overview ---

func TestHandleOverview_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type: got %s, want application/json", rec.Header().Get("Content-Type"))
	}
}

func TestHandleOverview_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestHandleOverview_DBError(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	mock.QueryErr = errFake

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestHandleOverview_Empty(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}

	var result []json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

func TestHandleOverview_WithData(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	mock.OverviewRows = []database.GetOverviewRow{
		{
			Hostname:   "test-server-1",
			ID:         pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
			Os:         pgtype.Text{String: "linux", Valid: true},
			Arch:       pgtype.Text{String: "amd64", Valid: true},
			Platform:   pgtype.Text{String: "ubuntu", Valid: true},
			CpuCores:   pgtype.Int4{Int32: 8, Valid: true},
			CpuUsage:   pgtype.Float8{Float64: 45.5, Valid: true},
			RamPercent: pgtype.Float8{Float64: 62.0, Valid: true},
			LastSeen:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
			UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			Uptime:     pgtype.Int8{Int64: 86400, Valid: true},
			Version:    "1.2.3",
		},
		{
			Hostname: "test-server-2",
		},
	}

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}

	var result []agentOverview
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(result))
	}

	if result[0].Hostname != "test-server-1" {
		t.Errorf("hostname: got %q, want test-server-1", result[0].Hostname)
	}
	if result[0].CPUUsage == nil || *result[0].CPUUsage != 45.5 {
		t.Errorf("cpu usage: got %v, want 45.5", result[0].CPUUsage)
	}
	if result[0].Version != "1.2.3" {
		t.Errorf("version: got %q, want 1.2.3", result[0].Version)
	}

	if result[1].Hostname != "test-server-2" {
		t.Errorf("hostname: got %q, want test-server-2", result[1].Hostname)
	}
	if result[1].CPUUsage != nil {
		t.Errorf("sparse agent cpu should be nil, got %v", result[1].CPUUsage)
	}
}

// --- List Agents ---

func TestHandleListAgents_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleListAgents_DBError(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	mock.QueryErr = errors.New("connection refused")

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

// --- Get Agent ---

func TestHandleGetAgent_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID, nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleGetAgent_InvalidID(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/invalid-uuid", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

// --- Delete Agent ---

func TestHandleDeleteAgent_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/agents/"+testUUID, nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", rec.Code)
	}
}

func TestHandleDeleteAgent_InvalidID(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/agents/invalid-uuid", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

// --- CPU Metrics ---

func TestHandleGetCPU_DefaultRange(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/cpu", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleGetCPU_QuickRange(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	ranges := []string{"5m", "15m", "1h", "6h", "24h", "7d", "30d"}
	for _, r := range ranges {
		req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/cpu?range="+r, nil))
		rec := httptest.NewRecorder()

		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("range=%s: status got %d, want 200", r, rec.Code)
		}
	}
}

func TestHandleGetCPU_InvalidRange(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/cpu?range=99h", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleGetCPU_CalendarRange(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	start := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	end := time.Now().Format(time.RFC3339)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/cpu?start="+start+"&end="+end, nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleGetCPU_CalendarStartOnly(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	start := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/cpu?start="+start, nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleGetCPU_InvalidStart(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/cpu?start=nodate", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleGetCPU_InvalidEnd(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	start := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/cpu?start="+start+"&end=nodate", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleGetCPU_StartAfterEnd(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	start := time.Now().Format(time.RFC3339)
	end := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/cpu?start="+start+"&end="+end, nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleGetCPU_InvalidAgentID(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/invalid/cpu", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

// --- All Metric Endpoints ---

func TestMetricEndpoints_AllReturn200(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	endpoints := []string{
		"/cpu", "/memory", "/disk", "/diskio", "/network",
		"/temperature", "/system", "/containers", "/wifi", "/pi",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+endpoint+"?range=1h", nil))
			rec := httptest.NewRecorder()

			s.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("%s: status: got %d, want 200", endpoint, rec.Code)
			}
		})
	}
}

// --- Metric Handlers ---

func TestMetricHandlers_BucketedAndError(t *testing.T) {
	tests := []struct {
		name, path, errField string
	}{
		{"Memory", "/api/v1/agents/{id}/memory", "QueryErr"},
		{"Disk", "/api/v1/agents/{id}/disk", "QueryErr"},
		{"DiskIO", "/api/v1/agents/{id}/diskio", "QueryErr"},
		{"Network", "/api/v1/agents/{id}/network", "QueryErr"},
		{"Temperature", "/api/v1/agents/{id}/temperature", "QueryErr"},
		{"System", "/api/v1/agents/{id}/system", "QueryErr"},
		{"Containers", "/api/v1/agents/{id}/containers", "QueryErr"},
		{"Wifi", "/api/v1/agents/{id}/wifi", "QueryErr"},
		{"Pi", "/api/v1/agents/{id}/pi", "QueryErr"},
	}

	agentID := "00000000-0000-0000-0000-000000000001"

	for _, tt := range tests {
		t.Run(tt.name+"_bucketed", func(t *testing.T) {
			s, _, _, mock := newTestServer()
			setupTestSession(mock)
			_ = mock

			path := strings.Replace(tt.path, "{id}", agentID, 1)
			req := authedRequest(httptest.NewRequest(http.MethodGet, path+"?range=7d", nil))
			rec := httptest.NewRecorder()

			s.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status: got %d, want 200", rec.Code)
			}
		})

		t.Run(tt.name+"_db_error", func(t *testing.T) {
			s, _, _, mock := newTestServer()
			setupTestSession(mock)
			mock.QueryErr = errors.New("connection refused")

			path := strings.Replace(tt.path, "{id}", agentID, 1)
			req := authedRequest(httptest.NewRequest(http.MethodGet, path+"?range=5m", nil))
			rec := httptest.NewRecorder()

			s.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusInternalServerError {
				t.Errorf("status: got %d, want 500", rec.Code)
			}
		})
	}
}

// --- Processes ---

func TestHandleGetProcesses_Default(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/processes", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleGetProcesses_SortMemory(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/processes?sort=memory", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleGetProcesses_CustomLimit(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/processes?limit=50", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleGetProcesses_InvalidLimit(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/processes?limit=abc", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleGetProcesses_LimitTooHigh(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/processes?limit=999", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

// --- Services, Applications, Updates ---

func TestHandleGetServices_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/services", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleGetApplications_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/applications", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleGetUpdates_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testUUID+"/updates", nil))
	rec := httptest.NewRecorder()

	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

// --- parseTimeRange (no auth needed, unit tests) ---

func TestParseTimeRange_QuickRanges(t *testing.T) {
	for label, d := range shortRanges {
		t.Run(label, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?range="+label, nil)
			start, end, err := parseTimeRange(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !start.Valid || !end.Valid {
				t.Fatal("start and end should be valid")
			}

			duration := end.Time.Sub(start.Time)
			tolerance := time.Second
			if d >= 24*time.Hour {
				tolerance = 2 * time.Hour // allow for DST
			}

			if diff := duration - d; diff < -tolerance || diff > tolerance {
				t.Errorf("duration = %v, want ~%v", duration, d)
			}
		})
	}
}

func TestParseTimeRange_InvalidRange(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?range=99h", nil)
	_, _, err := parseTimeRange(req)
	if err == nil {
		t.Error("expected error for invalid range")
	}
}

func TestParseTimeRange_Default(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	start, end, err := parseTimeRange(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	duration := end.Time.Sub(start.Time)
	if diff := duration - time.Hour; diff < -time.Second || diff > time.Second {
		t.Errorf("default duration = %v, want ~1h", duration)
	}
}

func TestParseTimeRange_CalendarRange(t *testing.T) {
	s := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	e := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodGet, "/?start="+s+"&end="+e, nil)
	start, end, err := parseTimeRange(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	duration := end.Time.Sub(start.Time)
	if diff := duration - time.Hour; diff < -2*time.Second || diff > 2*time.Second {
		t.Errorf("duration = %v, want ~1h", duration)
	}
}

func TestParseTimeRange_StartOnly(t *testing.T) {
	s := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodGet, "/?start="+s, nil)
	_, end, err := parseTimeRange(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diff := time.Since(end.Time); diff > 2*time.Second {
		t.Errorf("end should be ~now, got %v ago", diff)
	}
}

func TestParseTimeRange_InvalidStart(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?start=not-a-date", nil)
	_, _, err := parseTimeRange(req)
	if err == nil {
		t.Error("expected error for invalid start")
	}
}

func TestParseTimeRange_InvalidEnd(t *testing.T) {
	s := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/?start="+s+"&end=not-a-date", nil)
	_, _, err := parseTimeRange(req)
	if err == nil {
		t.Error("expected error for invalid end")
	}
}

func TestParseTimeRange_StartAfterEnd(t *testing.T) {
	s := time.Now().Format(time.RFC3339)
	e := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodGet, "/?start="+s+"&end="+e, nil)
	_, _, err := parseTimeRange(req)
	if err == nil {
		t.Error("expected error when start is after end")
	}
}

func TestParseTimeRange_ClampTo30Days(t *testing.T) {
	s := time.Now().AddDate(0, 0, -60).Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodGet, "/?start="+s, nil)
	start, _, err := parseTimeRange(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	oldest := time.Now().AddDate(0, 0, -30)
	if diff := start.Time.Sub(oldest); diff < -2*time.Second || diff > 2*time.Second {
		t.Errorf("start should be clamped to ~30 days ago, got %v", start.Time)
	}
}

func TestParseTimeRange_FutureEndClamped(t *testing.T) {
	s := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	e := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodGet, "/?start="+s+"&end="+e, nil)
	_, end, err := parseTimeRange(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diff := time.Since(end.Time); diff > 2*time.Second {
		t.Errorf("end should be clamped to ~now, got %v in the future", -diff)
	}
}

func TestCurrentStateHandlers(t *testing.T) {
	agentID := "00000000-0000-0000-0000-000000000001"

	endpoints := []struct {
		name, path string
	}{
		{"Services", "/api/v1/agents/" + agentID + "/services"},
		{"Applications", "/api/v1/agents/" + agentID + "/applications"},
		{"Updates", "/api/v1/agents/" + agentID + "/updates"},
		{"LatestSystem", "/api/v1/agents/" + agentID + "/system/latest"},
	}

	for _, ep := range endpoints {
		t.Run(ep.name+"_bad_id", func(t *testing.T) {
			s, _, _, mock := newTestServer()
			setupTestSession(mock)

			req := authedRequest(httptest.NewRequest(http.MethodGet, strings.Replace(ep.path, agentID, "invalid-uuid", 1), nil))
			rec := httptest.NewRecorder()

			s.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status: got %d, want 400", rec.Code)
			}
		})

		t.Run(ep.name+"_db_error", func(t *testing.T) {
			s, _, _, mock := newTestServer()
			setupTestSession(mock)
			mock.QueryErr = errors.New("connection refused")

			req := authedRequest(httptest.NewRequest(http.MethodGet, ep.path, nil))
			rec := httptest.NewRecorder()

			s.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusInternalServerError {
				t.Errorf("status: got %d, want 500", rec.Code)
			}
		})
	}
}

// --- Benchmarks ---

func BenchmarkHandleOverview(b *testing.B) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil))
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)
	}
}

func BenchmarkParseTimeRange_Quick(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/?range=1h", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		parseTimeRange(req)
	}
}

func BenchmarkParseTimeRange_Calendar(b *testing.B) {
	s := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	e := time.Now().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/?start="+s+"&end="+e, nil)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		parseTimeRange(req)
	}
}

func BenchmarkParseAgentID(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue("id", testUUID)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		parsePathID(req)
	}
}
