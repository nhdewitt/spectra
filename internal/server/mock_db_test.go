package server

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

// MockDB implements the DB interface for unit testing.
type MockDB struct {
	mu sync.Mutex

	// Stored agents: agentID (string) -> secret hash
	Agents map[string]string

	// Counters for verifying calls
	InsertCPUCount         int
	InsertMemoryCount      int
	InsertDiskCount        int
	InsertDiskIOCount      int
	InsertNetworkCount     int
	InsertTemperatureCount int
	InsertWifiCount        int
	InsertSystemCount      int
	InsertContainerCount   int
	InsertPiCount          int
	UpsertProcessCount     int
	UpsertServiceCount     int
	UpsertApplicationCount int
	TouchLastSeenCount     int

	Err error
}

func NewMockDB() *MockDB {
	return &MockDB{
		Agents: make(map[string]string),
	}
}

func (m *MockDB) RegisterAgent(_ context.Context, arg database.RegisterAgentParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Err != nil {
		return m.Err
	}

	id := uuidToString(arg.ID)
	m.Agents[id] = arg.SecretHash
	return nil
}

func (m *MockDB) GetAgentSecret(_ context.Context, id pgtype.UUID) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Err != nil {
		return "", m.Err
	}

	idStr := uuidToString(id)
	hash, ok := m.Agents[idStr]
	if !ok {
		return "", fmt.Errorf("agent not found")
	}
	return hash, nil
}

func (m *MockDB) TouchLastSeen(_ context.Context, _ pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TouchLastSeenCount++
	return m.Err
}

func (m *MockDB) AgentExists(_ context.Context, id pgtype.UUID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Err != nil {
		return false, m.Err
	}

	idStr := uuidToString(id)
	_, ok := m.Agents[idStr]
	return ok, nil
}

func (m *MockDB) InsertCPU(_ context.Context, _ database.InsertCPUParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InsertCPUCount++
	return m.Err
}

func (m *MockDB) InsertMemory(_ context.Context, _ database.InsertMemoryParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InsertMemoryCount++
	return m.Err
}

func (m *MockDB) InsertDisk(_ context.Context, _ database.InsertDiskParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InsertDiskCount++
	return m.Err
}

func (m *MockDB) InsertDiskIO(_ context.Context, _ database.InsertDiskIOParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InsertDiskIOCount++
	return m.Err
}

func (m *MockDB) InsertNetwork(_ context.Context, _ database.InsertNetworkParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InsertNetworkCount++
	return m.Err
}

func (m *MockDB) InsertTemperature(_ context.Context, _ database.InsertTemperatureParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InsertTemperatureCount++
	return m.Err
}

func (m *MockDB) InsertWifi(_ context.Context, _ database.InsertWifiParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InsertWifiCount++
	return m.Err
}

func (m *MockDB) InsertSystem(_ context.Context, _ database.InsertSystemParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InsertSystemCount++
	return m.Err
}

func (m *MockDB) InsertContainer(_ context.Context, _ database.InsertContainerParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InsertContainerCount++
	return m.Err
}

func (m *MockDB) InsertPi(_ context.Context, _ database.InsertPiParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InsertPiCount++
	return m.Err
}

func (m *MockDB) UpsertProcess(_ context.Context, _ database.UpsertProcessParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpsertProcessCount++
	return m.Err
}

func (m *MockDB) DeleteStaleProcesses(_ context.Context, _ database.DeleteStaleProcessesParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Err
}

func (m *MockDB) UpsertService(_ context.Context, _ database.UpsertServiceParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpsertServiceCount++
	return m.Err
}

func (m *MockDB) UpsertApplication(_ context.Context, _ database.UpsertApplicationParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpsertApplicationCount++
	return m.Err
}

func (m *MockDB) UpsertUpdates(_ context.Context, _ database.UpsertUpdatesParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Err
}

func (m *MockDB) UpsertCurrentCPU(_ context.Context, _ database.UpsertCurrentCPUParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Err
}

func (m *MockDB) UpsertCurrentMemory(_ context.Context, _ database.UpsertCurrentMemoryParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Err
}

func (m *MockDB) UpsertCurrentDiskMax(_ context.Context, _ pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Err
}

func (m *MockDB) UpsertCurrentNetwork(_ context.Context, _ pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Err
}

func (m *MockDB) UpsertCurrentTemperature(_ context.Context, _ pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Err
}

func (m *MockDB) UpsertCurrentSystem(_ context.Context, _ database.UpsertCurrentSystemParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Err
}

func (m *MockDB) UpsertCurrentReboot(_ context.Context, _ database.UpsertCurrentRebootParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Err
}

// uuidToString converts a pgtype.UUID to its string representation.
func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}

	b := u.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
