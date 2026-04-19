package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

func TestHandleFleetChart_DefaultMetric(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/overview/fleet/chart?range=1h", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string][]FleetChartPoint
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
}

func TestHandleFleetChart_ValidMetric(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	validMetrics := []string{"cpu", "mem", "disk"}

	for _, metric := range validMetrics {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/overview/fleet/chart?metric=%s&range=1h", metric), nil)
		req = authedRequest(req)
		w := httptest.NewRecorder()

		s.Router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", metric, w.Code)
		}
	}
}

func TestHandleFleetChart_InvalidMetric(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/overview/fleet/chart?metric=fake&range=1h", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleFleetChart_InvalidRange(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/overview/fleet/chart?range=10000d", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleFleetChart_DBError(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	mock.FleetErr = errFake

	req := httptest.NewRequest("GET", "/api/v1/overview/fleet/chart?metric=cpu&range=1h", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleFleetChart_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest("GET", "/api/v1/overview/fleet/chart?range=1h", nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleFleetHeatmap_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/overview/heatmap?range=24h", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []AgentHeatmap
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
}

func TestHandleFleetHeatmap_WithData(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	now := time.Now()
	uid := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}

	mock.HeatmapRows = []database.GetFleetHeatmapRow{
		{AgentID: uid, Bucket: pgtype.Timestamptz{Time: now, Valid: true}, Metric: "cpu", Value: 45.5},
		{AgentID: uid, Bucket: pgtype.Timestamptz{Time: now, Valid: true}, Metric: "mem", Value: 62.0},
		{AgentID: uid, Bucket: pgtype.Timestamptz{Time: now, Valid: true}, Metric: "disk", Value: 33.0},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview/heatmap?range=24h", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result []AgentHeatmap
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(result))
	}
	if len(result[0].CPU) != 1 {
		t.Errorf("expected 1 CPU cell, got %d", len(result[0].CPU))
	}
	if len(result[0].Mem) != 1 {
		t.Errorf("expected 1 Mem cell, got %d", len(result[0].Mem))
	}
	if len(result[0].Disk) != 1 {
		t.Errorf("expected 1 Disk cell, got %d", len(result[0].Disk))
	}
}

func TestHandleFleetHeatmap_InvalidRange(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/overview/heatmap?range=bogus", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleFleetHeatmap_DBError(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	mock.FleetErr = errFake

	req := httptest.NewRequest("GET", "/api/v1/overview/heatmap?range=24h", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleFleetHeatmap_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest("GET", "/api/v1/overview/heatmap?range=24h", nil)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleFleetHeatmap_DefaultRange(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := httptest.NewRequest("GET", "/api/v1/overview/heatmap", nil)
	req = authedRequest(req)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
