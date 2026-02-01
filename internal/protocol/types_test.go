package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPriorityToLevel(t *testing.T) {
	tests := []struct {
		priority int
		expected LogLevel
		exists   bool
	}{
		{0, LevelEmergency, true},
		{1, LevelAlert, true},
		{2, LevelCritical, true},
		{3, LevelError, true},
		{4, LevelWarning, true},
		{5, LevelNotice, true},
		{6, LevelInfo, true},
		{7, LevelDebug, true},
		{8, "", false},
		{-1, "", false},
	}

	for _, tt := range tests {
		level, exists := PriorityToLevel[tt.priority]
		if exists != tt.exists {
			t.Errorf("priority %d: exists = %v, want %v", tt.priority, exists, tt.exists)
		}
		if exists && level != tt.expected {
			t.Errorf("priority %d: level = %s, want %s", tt.priority, level, tt.expected)
		}
	}
}

func TestLogLevel_Constants(t *testing.T) {
	// Verify log levels are uppercase strings
	levels := map[LogLevel]string{
		LevelDebug:     "DEBUG",
		LevelInfo:      "INFO",
		LevelNotice:    "NOTICE",
		LevelWarning:   "WARNING",
		LevelError:     "ERROR",
		LevelCritical:  "CRITICAL",
		LevelAlert:     "ALERT",
		LevelEmergency: "EMERGENCY",
	}

	for level, expected := range levels {
		if string(level) != expected {
			t.Errorf("level %v should be %s", level, expected)
		}
	}
}

func TestCommandType_Constants(t *testing.T) {
	commands := []CommandType{
		CmdFetchLogs, CmdDiskUsage, CmdRestartAgent, CmdListMounts, CmdNetworkDiag,
	}

	seen := make(map[CommandType]bool)
	for _, cmd := range commands {
		if cmd == "" {
			t.Error("command type should not be empty")
		}
		if seen[cmd] {
			t.Errorf("duplicate command type: %s", cmd)
		}
		seen[cmd] = true
	}
}

func TestProcStatus_Constants(t *testing.T) {
	statuses := []ProcStatus{
		ProcRunning, ProcRunnable, ProcWaiting, ProcOther,
	}

	seen := make(map[ProcStatus]bool)
	for _, status := range statuses {
		if status == "" {
			t.Error("proc status should not be empty")
		}
		if seen[status] {
			t.Errorf("duplicate proc status: %s", status)
		}
		seen[status] = true
	}
}

func TestCommand_JSON(t *testing.T) {
	cmd := Command{
		ID:      "cmd-123",
		Type:    CmdFetchLogs,
		Payload: []byte(`{"min_level":"ERROR"}`),
	}

	b, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded Command
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.ID != cmd.ID {
		t.Errorf("ID: got %s, want %s", decoded.ID, cmd.ID)
	}
	if decoded.Type != cmd.Type {
		t.Errorf("Type: got %s, want %s", decoded.Type, cmd.Type)
	}
}

func TestCommandResult_JSON(t *testing.T) {
	result := CommandResult{
		ID:      "cmd-123",
		Type:    CmdFetchLogs,
		Payload: json.RawMessage(`{"logs":[]}`),
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded CommandResult
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.ID != result.ID {
		t.Errorf("ID: got %s, want %s", decoded.ID, result.ID)
	}
	if string(decoded.Payload) != string(result.Payload) {
		t.Errorf("Payload: got %s, want %s", decoded.Payload, result.Payload)
	}
}

func TestCommandResult_WithError(t *testing.T) {
	result := CommandResult{
		ID:    "cmd-123",
		Type:  CmdNetworkDiag,
		Error: "connection refused",
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var data map[string]any
	json.Unmarshal(b, &data)

	if data["error"] != "connection refused" {
		t.Errorf("error: got %v, want 'connection refused'", data["error"])
	}
}

func TestMetric_OmitEmpty(t *testing.T) {
	metric := DiskMetric{
		Device:     "/dev/sda1",
		Mountpoint: "/",
		Total:      100,
		Used:       50,
		// InodesTotal, InodesUsed, InodesPct should be omitted
	}

	b, _ := json.Marshal(metric)
	s := string(b)

	if strings.Contains(s, "inodes_total") {
		t.Error("inodes_total should be omitted when zero")
	}
	if strings.Contains(s, "inodes_used") {
		t.Error("inodes_used should be omitted when zero")
	}
	if strings.Contains(s, "inodes_pct") {
		t.Error("inodes_pct should be omitted when zero")
	}
}

func TestProcessMetric_OptionalFields(t *testing.T) {
	running := uint32(5)
	// With optional fields
	metric := ProcessMetric{
		Pid:            1234,
		Name:           "test",
		ThreadsTotal:   10,
		ThreadsRunning: &running,
	}

	b, _ := json.Marshal(metric)
	s := string(b)

	if !strings.Contains(s, "threads_running") {
		t.Error("threads_running should be present when set")
	}

	// Without optional fields
	metric2 := ProcessMetric{
		Pid:          1234,
		Name:         "test",
		ThreadsTotal: 10,
	}

	b2, _ := json.Marshal(metric2)
	s2 := string(b2)

	if strings.Contains(s2, "threads_running") {
		t.Error("threads_running should be omitted when nil")
	}
}

func BenchmarkEnvelope_Marshal_CPU(b *testing.B) {
	env := Envelope{
		Type:      "cpu",
		Timestamp: time.Now(),
		Hostname:  "test-host",
		Data:      CPUMetric{Usage: 75.5, CoreUsage: []float64{80, 70, 85, 65}},
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _ = json.Marshal(env)
	}
}

func BenchmarkEnvelope_Marshal_ProcessList(b *testing.B) {
	procs := make([]ProcessMetric, 100)
	for i := range procs {
		procs[i] = ProcessMetric{
			Pid:          i,
			Name:         "process",
			CPUPercent:   1.5,
			MemPercent:   2.5,
			MemRSS:       1000000,
			Status:       ProcRunning,
			ThreadsTotal: 10,
		}
	}

	env := Envelope{
		Type:      "process_list",
		Timestamp: time.Now(),
		Hostname:  "test-host",
		Data:      ProcessListMetric{Processes: procs},
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _ = json.Marshal(env)
	}
}

func BenchmarkEnvelope_Marshal_ContainerList(b *testing.B) {
	containers := make([]ContainerMetric, 20)
	for i := range containers {
		containers[i] = ContainerMetric{
			ID:          "abc123",
			Name:        "container",
			State:       "running",
			Source:      "docker",
			Kind:        "container",
			CPUPercent:  50.0,
			MemoryBytes: 1000000000,
		}
	}

	env := Envelope{
		Type:      "container_list",
		Timestamp: time.Now(),
		Hostname:  "test-host",
		Data:      ContainerListMetric{Containers: containers},
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _ = json.Marshal(env)
	}
}
