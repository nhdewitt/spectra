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

	id := formatUUID(arg.ID)
	m.Agents[id] = arg.SecretHash
	return nil
}

func (m *MockDB) GetAgentSecret(_ context.Context, id pgtype.UUID) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Err != nil {
		return "", m.Err
	}

	idStr := formatUUID(id)
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

	idStr := formatUUID(id)
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

func (m *MockDB) GetOverview(_ context.Context) ([]database.GetOverviewRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.GetOverviewRow{}, m.Err
}

func (m *MockDB) GetAgent(_ context.Context, _ pgtype.UUID) (database.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return database.Agent{}, nil
}

func (m *MockDB) DeleteAgent(_ context.Context, _ pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *MockDB) GetCPURange(_ context.Context, _ database.GetCPURangeParams) ([]database.MetricsCpu, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.MetricsCpu{}, nil
}

func (m *MockDB) GetMemoryRange(_ context.Context, _ database.GetMemoryRangeParams) ([]database.MetricsMemory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.MetricsMemory{}, nil
}

func (m *MockDB) GetDiskRange(_ context.Context, _ database.GetDiskRangeParams) ([]database.MetricsDisk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.MetricsDisk{}, nil
}

func (m *MockDB) GetDiskIORange(_ context.Context, _ database.GetDiskIORangeParams) ([]database.MetricsDiskIo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.MetricsDiskIo{}, nil
}

func (m *MockDB) GetNetworkRange(_ context.Context, _ database.GetNetworkRangeParams) ([]database.MetricsNetwork, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.MetricsNetwork{}, nil
}

func (m *MockDB) GetTemperatureRange(_ context.Context, _ database.GetTemperatureRangeParams) ([]database.MetricsTemperature, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.MetricsTemperature{}, nil
}

func (m *MockDB) GetSystemRange(_ context.Context, _ database.GetSystemRangeParams) ([]database.MetricsSystem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.MetricsSystem{}, nil
}

func (m *MockDB) GetContainerRange(_ context.Context, _ database.GetContainerRangeParams) ([]database.MetricsContainer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.MetricsContainer{}, nil
}

func (m *MockDB) GetWifiRange(_ context.Context, _ database.GetWifiRangeParams) ([]database.MetricsWifi, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.MetricsWifi{}, nil
}

func (m *MockDB) GetPiRange(_ context.Context, _ database.GetPiRangeParams) ([]database.GetPiRangeRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.GetPiRangeRow{}, nil
}

func (m *MockDB) GetProcessesByCPU(_ context.Context, _ database.GetProcessesByCPUParams) ([]database.CurrentProcess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.CurrentProcess{}, nil
}

func (m *MockDB) GetProcessesByMemory(_ context.Context, _ database.GetProcessesByMemoryParams) ([]database.CurrentProcess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.CurrentProcess{}, nil
}

func (m *MockDB) GetServices(_ context.Context, _ pgtype.UUID) ([]database.CurrentService, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.CurrentService{}, nil
}

func (m *MockDB) GetApplications(_ context.Context, _ pgtype.UUID) ([]database.CurrentApplication, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.CurrentApplication{}, nil
}

func (m *MockDB) GetUpdates(_ context.Context, _ pgtype.UUID) (database.CurrentUpdate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return database.CurrentUpdate{}, nil
}

func (m *MockDB) ListAgents(_ context.Context) ([]database.ListAgentsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return []database.ListAgentsRow{}, nil
}
