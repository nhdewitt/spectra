package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

func TestHandleGetSparklines_Empty(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview/sparklines", nil)
	rec := httptest.NewRecorder()
	s.handleGetSparklines(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var result map[string]*sparklineData
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestHandleGetSparklines_WithData(t *testing.T) {
	s, _, _, mock := newTestServer()

	agentUUID := pgtype.UUID{
		Bytes: [16]byte{
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		},
		Valid: true,
	}

	mock.mu.Lock()
	mock.recentCPU = []database.GetRecentCPURow{
		{AgentID: agentUUID, Usage: pgtype.Float8{Float64: 15.5, Valid: true}},
		{AgentID: agentUUID, Usage: pgtype.Float8{Float64: 22.3, Valid: true}},
		{AgentID: agentUUID, Usage: pgtype.Float8{Float64: 18.1, Valid: true}},
	}
	mock.recentMemory = []database.GetRecentMemoryRow{
		{AgentID: agentUUID, RamPercent: pgtype.Float8{Float64: 45.0, Valid: true}},
		{AgentID: agentUUID, RamPercent: pgtype.Float8{Float64: 46.2, Valid: true}},
	}
	mock.recentDiskMax = []database.GetRecentDiskMaxRow{
		{AgentID: agentUUID, MaxPercent: 72.5},
		{AgentID: agentUUID, MaxPercent: 72.6},
		{AgentID: agentUUID, MaxPercent: 72.6},
	}
	mock.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview/sparklines", nil)
	rec := httptest.NewRecorder()
	s.handleGetSparklines(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var result map[string]*sparklineData
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	agentID := formatUUID(agentUUID)
	data, ok := result[agentID]
	if !ok {
		t.Fatalf("agent %s not in result", agentID)
	}

	if len(data.CPU) != 3 {
		t.Errorf("cpu points = %d, want 3", len(data.CPU))
	}
	if len(data.Mem) != 2 {
		t.Errorf("mem points = %d, want 2", len(data.Mem))
	}
	if len(data.Disk) != 3 {
		t.Errorf("disk points = %d, want 3", len(data.Disk))
	}

	// Verify values
	if data.CPU[0] != 15.5 {
		t.Errorf("cpu[0] = %f, want 15.5", data.CPU[0])
	}
	if data.Mem[1] != 46.2 {
		t.Errorf("mem[1] = %f, want 46.2", data.Mem[1])
	}
	if data.Disk[0] != 72.5 {
		t.Errorf("disk[0] = %f, want 72.5", data.Disk[0])
	}
}

func TestHandleGetSparklines_MultipleAgents(t *testing.T) {
	s, _, _, mock := newTestServer()

	agent1 := pgtype.UUID{
		Bytes: [16]byte{
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		},
		Valid: true,
	}
	agent2 := pgtype.UUID{
		Bytes: [16]byte{
			0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
			0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
		},
		Valid: true,
	}

	mock.mu.Lock()
	mock.recentCPU = []database.GetRecentCPURow{
		{AgentID: agent1, Usage: pgtype.Float8{Float64: 10.0, Valid: true}},
		{AgentID: agent2, Usage: pgtype.Float8{Float64: 90.0, Valid: true}},
	}
	mock.recentMemory = []database.GetRecentMemoryRow{
		{AgentID: agent1, RamPercent: pgtype.Float8{Float64: 30.0, Valid: true}},
		{AgentID: agent2, RamPercent: pgtype.Float8{Float64: 80.0, Valid: true}},
	}
	mock.recentDiskMax = []database.GetRecentDiskMaxRow{
		{AgentID: agent1, MaxPercent: 50.0},
		{AgentID: agent2, MaxPercent: 95.0},
	}
	mock.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview/sparklines", nil)
	rec := httptest.NewRecorder()
	s.handleGetSparklines(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var result map[string]*sparklineData
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 agents, got %d", len(result))
	}

	id1 := formatUUID(agent1)
	id2 := formatUUID(agent2)

	if result[id1].CPU[0] != 10.0 {
		t.Errorf("agent1 cpu = %f, want 10.0", result[id1].CPU[0])
	}
	if result[id2].CPU[0] != 90.0 {
		t.Errorf("agent2 cpu = %f, want 90.0", result[id2].CPU[0])
	}
}

func TestHandleGetSparklines_EmptyArraysNotNull(t *testing.T) {
	s, _, _, mock := newTestServer()

	agentUUID := pgtype.UUID{
		Bytes: [16]byte{
			0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
		},
		Valid: true,
	}

	// Only CPU data, no mem or disk
	mock.mu.Lock()
	mock.recentCPU = []database.GetRecentCPURow{
		{AgentID: agentUUID, Usage: pgtype.Float8{Float64: 5.0, Valid: true}},
	}
	mock.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview/sparklines", nil)
	rec := httptest.NewRecorder()
	s.handleGetSparklines(rec, req)

	// Verify JSON has [] not null for empty arrays
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	agentID := formatUUID(agentUUID)
	agentRaw, ok := raw[agentID]
	if !ok {
		t.Fatalf("agent not in result")
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(agentRaw, &data); err != nil {
		t.Fatalf("decode agent error: %v", err)
	}

	// mem and disk should be [] not null
	if string(data["mem"]) == "null" {
		t.Error("mem should be [] not null")
	}
	if string(data["disk"]) == "null" {
		t.Error("disk should be [] not null")
	}
}
