package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
	"golang.org/x/crypto/bcrypt"
)

const (
	// testSessionToken is the session token used by authedRequest.
	testSessionToken = "test-session-token"
	// testSessionIP is the IP address bound to the test session.
	testSessionIP = "192.168.1.100"
)

// setupTestSession creates a valid session in the mock DB for use in tests
// that route through s.Router.ServeHTTP (which hits requireUserAuth).
// Call this once after newTestServer()
func setupTestSession(mock *MockDB) {
	mock.AddSession(testSessionToken, "testadmin", "admin", testSessionIP)
}

// authedRequest attaches the test session cookie and matching RemoteAddr
// to an *http.Request so it passes through requireUserAuth middleware.
func authedRequest(req *http.Request) *http.Request {
	req.RemoteAddr = testSessionIP + ":12345"
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: testSessionToken,
	})
	return req
}

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

	// Auth
	Users    map[string]mockUser    // username -> user
	Sessions map[string]mockSession // token -> session

	// Sparkline seed data
	recentCPU     []database.GetRecentCPURow
	recentMemory  []database.GetRecentMemoryRow
	recentDiskMax []database.GetRecentDiskMaxRow

	Err      error
	QueryErr error // errors for data queries (not auth)
}

type mockUser struct {
	ID       pgtype.UUID
	Username string
	Password string // bcrypt hash
	Role     string
}

type mockSession struct {
	UserID   pgtype.UUID
	Username string
	Role     string
	//nolint:revive // sqlc uses IpAddress for ip_address column
	IpAddress string
}

func NewMockDB() *MockDB {
	return &MockDB{
		Agents:   make(map[string]string),
		Users:    make(map[string]mockUser),
		Sessions: make(map[string]mockSession),
	}
}

func (m *MockDB) AddUser(username, password, role string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	m.Users[username] = mockUser{
		Username: username,
		Password: string(hash),
		Role:     role,
	}
}

func (m *MockDB) AddSession(token, username, role, ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Sessions[token] = mockSession{
		Username:  username,
		Role:      role,
		IpAddress: ip,
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
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
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

func (m *MockDB) CreateUser(_ context.Context, args database.CreateUserParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Err != nil {
		return m.Err
	}

	m.Users[args.Username] = mockUser{
		Username: args.Username,
		Password: args.Password,
		Role:     args.Role,
	}

	return nil
}

func (m *MockDB) GetUserByUsername(_ context.Context, username string) (database.GetUserByUsernameRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Err != nil {
		return database.GetUserByUsernameRow{}, m.Err
	}

	u, ok := m.Users[username]
	if !ok {
		return database.GetUserByUsernameRow{}, fmt.Errorf("user not found")
	}
	return database.GetUserByUsernameRow{
		ID:       u.ID,
		Username: u.Username,
		Password: u.Password,
		Role:     u.Role,
	}, nil
}

func (m *MockDB) UserCount(_ context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Err != nil {
		return 0, m.Err
	}
	return int64(len(m.Users)), nil
}

func (m *MockDB) CreateSession(_ context.Context, args database.CreateSessionParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Err != nil {
		return m.Err
	}

	m.Sessions[args.Token] = mockSession{
		UserID: args.UserID,
	}

	return nil
}

func (m *MockDB) GetSession(_ context.Context, token string) (database.GetSessionRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Err != nil {
		return database.GetSessionRow{}, m.Err
	}

	s, ok := m.Sessions[token]
	if !ok {
		return database.GetSessionRow{}, fmt.Errorf("session not found")
	}

	return database.GetSessionRow{
		Token:     token,
		UserID:    s.UserID,
		Username:  s.Username,
		Role:      s.Role,
		IpAddress: s.IpAddress,
	}, nil
}

func (m *MockDB) DeleteSession(_ context.Context, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *MockDB) DeleteExpiredSessions(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *MockDB) DeleteUserSessions(_ context.Context, _ pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *MockDB) GetRecentCPU(_ context.Context) ([]database.GetRecentCPURow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.recentCPU != nil {
		return m.recentCPU, nil
	}
	return []database.GetRecentCPURow{}, nil
}

func (m *MockDB) GetRecentMemory(_ context.Context) ([]database.GetRecentMemoryRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.recentMemory != nil {
		return m.recentMemory, nil
	}
	return []database.GetRecentMemoryRow{}, nil
}

func (m *MockDB) GetRecentDiskMax(_ context.Context) ([]database.GetRecentDiskMaxRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.recentDiskMax != nil {
		return m.recentDiskMax, nil
	}
	return []database.GetRecentDiskMaxRow{}, nil
}
