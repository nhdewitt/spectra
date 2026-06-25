package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
	"golang.org/x/crypto/bcrypt"
)

const (
	// testSessionToken is the session token used by authedRequest.
	testSessionToken = "test-session-token"
	// testSessionIP is the IP address bound to the test session.
	testSessionIP = "192.168.1.100"
	// testAgentID is a fixed, regex-valid UUID for tests.
	testAgentUUID = "550e8400-e29b-41d4-a716-446655440000"
)

// setupTestSession creates a valid session in the mock DB for use in tests
// that route through s.Router.ServeHTTP (which hits requireUserAuth).
// Call this once after newTestServer()
func setupTestSession(mock *MockDB) {
	mock.AddSession(testSessionToken, "testadmin", "admin", testSessionIP)
}

// setupTestSessionWithRole creates a session with a specific role and known user ID.
func setupTestSessionWithRole(mock *MockDB, token, username, role, ip string, userID pgtype.UUID) {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	mock.Sessions[token] = mockSession{
		UserID:    userID,
		Username:  username,
		Role:      role,
		IpAddress: ip,
	}
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

// authedRequestAs creates a request with a specific session token and IP.
func authedRequestAs(req *http.Request, token, ip string) *http.Request {
	req.RemoteAddr = ip + ":12345"
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: token,
	})
	return req
}

func tsNow() pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Now(), Valid: true}
}

func putBody(value string) *bytes.Buffer {
	return bytes.NewBufferString(fmt.Sprintf(`{"value":%q}`, value))
}

func newTestUUID() pgtype.UUID {
	var id pgtype.UUID

	if _, err := rand.Read(id.Bytes[:]); err != nil {
		panic(err)
	}

	id.Bytes[6] = (id.Bytes[6] & 0x0f) | 0x40
	id.Bytes[8] = (id.Bytes[8] & 0x3f) | 0x80
	id.Valid = true
	return id
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
	Users       map[string]mockUser    // username -> user
	Sessions    map[string]mockSession // token -> session
	AgentSHA256 map[string][]byte      // agentID -> sha256 hash

	// Sparkline seed data
	recentCPU     []database.GetRecentCPURow
	recentMemory  []database.GetRecentMemoryRow
	recentDiskMax []database.GetRecentDiskMaxRow

	OverviewRows []database.GetOverviewRow
	HeatmapRows  []database.GetFleetHeatmapRow

	// Labels
	ReplaceAutoLabelsCount      int
	LastReplaceAutoLabelsParams database.ReplaceAutoLabelsParams
	UpsertAutoLabelCount        int
	LastUpsertAutoLabelParams   database.UpsertAutoLabelParams
	ListAgentLabelsCount        int
	ListAgentLabelsReturn       []database.ListAgentLabelsRow
	ListLabelKeysCount          int
	ListLabelKeysReturn         []database.ListLabelKeysRow
	ListLabelValuesForKeyCount  int
	ListLabelValuesForKeyReturn []string
	UpsertUserLabelCount        int
	LastUpsertUserLabelParams   database.UpsertUserLabelParams
	UpsertUserLabelReturn       database.AgentLabel
	UpsertUserLabelErr          error
	DeleteUserLabelCount        int
	DeleteUserLabelErr          error
	LastDeleteUserLabelParams   database.DeleteUserLabelParams
	DeleteUserLabelRows         int64
	GetAgentLabelCount          int
	LastGetAgentLabelParams     database.GetAgentLabelParams
	GetAgentLabelReturn         database.GetAgentLabelRow
	GetAgentLabelErr            error

	UserRows              []database.ListUsersRow
	UserRowsWithLastLogin []database.ListUsersWithLastLoginRow
	UserByID              map[pgtype.UUID]database.GetUserByIDRow
	SuperAdmins           int64
	DeleteUserCount       int
	UpdateUserRoleCount   int
	OfflineAgentCount     int64

	AlertChannels   map[string]database.AlertChannel // UUID -> channel
	AlertChannelErr error

	AlertRules   map[string]database.AlertRule // UUID -> rule
	AlertRuleErr error

	RuleChannels map[string][]pgtype.UUID // rule id -> []channel id

	AlertEvents   map[string]database.AlertEvent // id -> event
	AlertEventErr error

	DiskTrendRows []database.GetDiskTrendRow
	DiskTrendErr  error

	AllServices []database.CurrentService // Bulk preload

	ServicesByAgent map[string][]database.CurrentService // agentID -> services

	SMTPConfig    *database.SmtpConfig
	SMTPConfigErr error

	ListAllAgentLabelsReturn []database.ListAllAgentLabelsRow

	Err         error
	QueryErr    error // errors for data queries (not auth)
	GetAgentErr error
	FleetErr    error // errors for fleet queries
	ConfigErr   error // errors for agent config queries
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
		Agents:          make(map[string]string),
		Users:           make(map[string]mockUser),
		Sessions:        make(map[string]mockSession),
		AgentSHA256:     make(map[string][]byte),
		SuperAdmins:     1,
		AlertChannels:   make(map[string]database.AlertChannel),
		AlertRules:      make(map[string]database.AlertRule),
		RuleChannels:    make(map[string][]pgtype.UUID),
		AlertEvents:     make(map[string]database.AlertEvent),
		ServicesByAgent: make(map[string][]database.CurrentService),
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
	return m.OverviewRows, m.Err
}

func (m *MockDB) GetAgent(_ context.Context, _ pgtype.UUID) (database.GetAgentRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.GetAgentErr != nil {
		return database.GetAgentRow{}, m.GetAgentErr
	}
	return database.GetAgentRow{}, nil
}

func (m *MockDB) DeleteAgent(_ context.Context, _ pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *MockDB) GetCPURange(_ context.Context, _ database.GetCPURangeParams) ([]database.MetricsCpu, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.MetricsCpu{}, nil
}

func (m *MockDB) GetMemoryRange(_ context.Context, _ database.GetMemoryRangeParams) ([]database.MetricsMemory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.MetricsMemory{}, nil
}

func (m *MockDB) GetDiskRange(_ context.Context, _ database.GetDiskRangeParams) ([]database.MetricsDisk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.MetricsDisk{}, nil
}

func (m *MockDB) GetDiskIORange(_ context.Context, _ database.GetDiskIORangeParams) ([]database.MetricsDiskIo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.MetricsDiskIo{}, nil
}

func (m *MockDB) GetNetworkRange(_ context.Context, _ database.GetNetworkRangeParams) ([]database.MetricsNetwork, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.MetricsNetwork{}, nil
}

func (m *MockDB) GetTemperatureRange(_ context.Context, _ database.GetTemperatureRangeParams) ([]database.MetricsTemperature, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.MetricsTemperature{}, nil
}

func (m *MockDB) GetSystemRange(_ context.Context, _ database.GetSystemRangeParams) ([]database.MetricsSystem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.MetricsSystem{}, nil
}

func (m *MockDB) GetContainerRange(_ context.Context, _ database.GetContainerRangeParams) ([]database.MetricsContainer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.MetricsContainer{}, nil
}

func (m *MockDB) GetWifiRange(_ context.Context, _ database.GetWifiRangeParams) ([]database.MetricsWifi, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.MetricsWifi{}, nil
}

func (m *MockDB) GetPiRange(_ context.Context, _ database.GetPiRangeParams) ([]database.GetPiRangeRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.GetPiRangeRow{}, nil
}

func (m *MockDB) GetCPUBucketed(_ context.Context, _ database.GetCPUBucketedParams) ([]database.GetCPUBucketedRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.GetCPUBucketedRow{}, nil
}

func (m *MockDB) GetMemoryBucketed(_ context.Context, _ database.GetMemoryBucketedParams) ([]database.GetMemoryBucketedRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.GetMemoryBucketedRow{}, nil
}

func (m *MockDB) GetDiskBucketed(_ context.Context, _ database.GetDiskBucketedParams) ([]database.GetDiskBucketedRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.GetDiskBucketedRow{}, nil
}

func (m *MockDB) GetDiskIOBucketed(_ context.Context, _ database.GetDiskIOBucketedParams) ([]database.GetDiskIOBucketedRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.GetDiskIOBucketedRow{}, nil
}

func (m *MockDB) GetNetworkBucketed(_ context.Context, _ database.GetNetworkBucketedParams) ([]database.GetNetworkBucketedRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.GetNetworkBucketedRow{}, nil
}

func (m *MockDB) GetTemperatureBucketed(_ context.Context, _ database.GetTemperatureBucketedParams) ([]database.GetTemperatureBucketedRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.GetTemperatureBucketedRow{}, nil
}

func (m *MockDB) GetSystemBucketed(_ context.Context, _ database.GetSystemBucketedParams) ([]database.GetSystemBucketedRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.GetSystemBucketedRow{}, nil
}

func (m *MockDB) GetContainerBucketed(_ context.Context, _ database.GetContainerBucketedParams) ([]database.GetContainerBucketedRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.GetContainerBucketedRow{}, nil
}

func (m *MockDB) GetWifiBucketed(_ context.Context, _ database.GetWifiBucketedParams) ([]database.GetWifiBucketedRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.GetWifiBucketedRow{}, nil
}

func (m *MockDB) GetPiBucketed(_ context.Context, _ database.GetPiBucketedParams) ([]database.GetPiBucketedRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.GetPiBucketedRow{}, nil
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

func (m *MockDB) GetServices(_ context.Context, id pgtype.UUID) ([]database.CurrentService, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	if svcs, ok := m.ServicesByAgent[formatUUID(id)]; ok {
		return svcs, nil
	}
	return []database.CurrentService{}, nil
}

func (m *MockDB) GetApplications(_ context.Context, _ pgtype.UUID) ([]database.CurrentApplication, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.CurrentApplication{}, nil
}

func (m *MockDB) GetUpdates(_ context.Context, _ pgtype.UUID) (database.CurrentUpdate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return database.CurrentUpdate{}, m.QueryErr
	}
	return database.CurrentUpdate{}, nil
}

func (m *MockDB) ListAgents(_ context.Context) ([]database.ListAgentsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
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

func (m *MockDB) GetAgentSecretSHA256(_ context.Context, id pgtype.UUID) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	idStr := formatUUID(id)
	hash, ok := m.AgentSHA256[idStr]
	if !ok {
		return nil, fmt.Errorf("no sha256 hash for agent")
	}
	return hash, nil
}

func (m *MockDB) SetAgentSecretSHA256(_ context.Context, args database.SetAgentSecretSHA256Params) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	idStr := formatUUID(args.ID)
	m.AgentSHA256[idStr] = args.SecretSha256
	return nil
}

func (m *MockDB) TouchLastSeenIfStale(_ context.Context, _ database.TouchLastSeenIfStaleParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TouchLastSeenCount++
	return m.Err
}

func (m *MockDB) GetLatestSystem(_ context.Context, _ pgtype.UUID) (database.MetricsSystem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return database.MetricsSystem{}, m.QueryErr
	}
	return database.MetricsSystem{}, nil
}

func (m *MockDB) GetAgentConfig(_ context.Context, _ pgtype.UUID) ([]database.AgentConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return []database.AgentConfig{}, nil
}

func (m *MockDB) GetAgentConfigByKey(_ context.Context, _ database.GetAgentConfigByKeyParams) (database.AgentConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return database.AgentConfig{}, nil
}

func (m *MockDB) SetAgentConfig(_ context.Context, _ database.SetAgentConfigParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ConfigErr != nil {
		return m.ConfigErr
	}
	return nil
}

func (m *MockDB) DeleteAgentConfig(_ context.Context, _ database.DeleteAgentConfigParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ConfigErr != nil {
		return m.ConfigErr
	}
	return m.Err
}

func (m *MockDB) DeleteAllAgentConfig(_ context.Context, _ pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ConfigErr != nil {
		return m.ConfigErr
	}
	return m.Err
}

func (m *MockDB) GetFleetHeatmap(_ context.Context, _ database.GetFleetHeatmapParams) ([]database.GetFleetHeatmapRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FleetErr != nil {
		return nil, m.FleetErr
	}
	return m.HeatmapRows, nil
}

func (m *MockDB) GetFleetSparkCPU(_ context.Context, _ database.GetFleetSparkCPUParams) ([]database.GetFleetSparkCPURow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FleetErr != nil {
		return nil, m.FleetErr
	}
	return []database.GetFleetSparkCPURow{}, nil
}

func (m *MockDB) GetFleetSparkMemory(_ context.Context, _ database.GetFleetSparkMemoryParams) ([]database.GetFleetSparkMemoryRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FleetErr != nil {
		return nil, m.FleetErr
	}
	return []database.GetFleetSparkMemoryRow{}, nil
}

func (m *MockDB) GetFleetSparkDisk(_ context.Context, _ database.GetFleetSparkDiskParams) ([]database.GetFleetSparkDiskRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FleetErr != nil {
		return nil, m.FleetErr
	}
	return []database.GetFleetSparkDiskRow{}, nil
}

func (m *MockDB) UpdateAgentVersion(_ context.Context, _ database.UpdateAgentVersionParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Err
}

func (m *MockDB) ListUsers(_ context.Context) ([]database.ListUsersRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return m.UserRows, nil
}

func (m *MockDB) GetUserByID(_ context.Context, id pgtype.UUID) (database.GetUserByIDRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return database.GetUserByIDRow{}, m.QueryErr
	}
	if m.UserByID != nil {
		if row, ok := m.UserByID[id]; ok {
			return row, nil
		}
	}
	return database.GetUserByIDRow{}, fmt.Errorf("user not found")
}

func (m *MockDB) SuperAdminCount(_ context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return 0, m.QueryErr
	}
	return m.SuperAdmins, nil
}

func (m *MockDB) DeleteUser(_ context.Context, _ pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteUserCount++
	return m.Err
}

func (m *MockDB) UpdateUserRole(_ context.Context, _ database.UpdateUserRoleParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpdateUserRoleCount++
	return m.Err
}

func (m *MockDB) PurgeOfflineAgents(_ context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return 0, m.QueryErr
	}
	return m.OfflineAgentCount, nil
}

func (m *MockDB) GetUserConfig(_ context.Context, _ pgtype.UUID) ([]database.GetUserConfigRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return nil, m.Err
}

func (m *MockDB) SetUserConfig(_ context.Context, _ database.SetUserConfigParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return m.QueryErr
	}
	return m.Err
}

func (m *MockDB) DeleteUserConfig(_ context.Context, _ database.DeleteUserConfigParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return m.QueryErr
	}
	return m.Err
}

func (m *MockDB) ListUsersWithLastLogin(_ context.Context) ([]database.ListUsersWithLastLoginRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return m.UserRowsWithLastLogin, nil
}

func (m *MockDB) UpsertSuperadmin(_ context.Context, _ database.UpsertSuperadminParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Err
}

func (m *MockDB) ReplaceAutoLabels(_ context.Context, arg database.ReplaceAutoLabelsParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ReplaceAutoLabelsCount++
	m.LastReplaceAutoLabelsParams = arg
	return m.Err
}

func (m *MockDB) UpsertAutoLabel(_ context.Context, arg database.UpsertAutoLabelParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpsertAutoLabelCount++
	m.LastUpsertAutoLabelParams = arg
	return m.Err
}

func (m *MockDB) ListAgentLabels(_ context.Context, _ pgtype.UUID) ([]database.ListAgentLabelsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListAgentLabelsCount++
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	if m.Err != nil {
		return nil, m.Err
	}
	return m.ListAgentLabelsReturn, nil
}

func (m *MockDB) ListLabelKeys(_ context.Context) ([]database.ListLabelKeysRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	if m.Err != nil {
		return nil, m.Err
	}
	return m.ListLabelKeysReturn, nil
}

func (m *MockDB) ListLabelValuesForKey(_ context.Context, _ string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListLabelValuesForKeyCount++
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	if m.Err != nil {
		return nil, m.Err
	}
	return m.ListLabelValuesForKeyReturn, nil
}

func (m *MockDB) UpsertUserLabel(_ context.Context, arg database.UpsertUserLabelParams) (database.AgentLabel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpsertUserLabelCount++
	m.LastUpsertUserLabelParams = arg
	if m.UpsertUserLabelErr != nil {
		return database.AgentLabel{}, m.UpsertUserLabelErr
	}
	if m.Err != nil {
		return database.AgentLabel{}, m.Err
	}
	return m.UpsertUserLabelReturn, nil
}

func (m *MockDB) GetAgentLabel(_ context.Context, arg database.GetAgentLabelParams) (database.GetAgentLabelRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetAgentLabelCount++
	m.LastGetAgentLabelParams = arg
	if m.GetAgentLabelErr != nil {
		return database.GetAgentLabelRow{}, m.GetAgentLabelErr
	}
	if m.Err != nil {
		return database.GetAgentLabelRow{}, m.Err
	}
	return m.GetAgentLabelReturn, nil
}

func (m *MockDB) DeleteUserLabel(_ context.Context, arg database.DeleteUserLabelParams) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteUserLabelCount++
	m.LastDeleteUserLabelParams = arg
	if m.DeleteUserLabelErr != nil {
		return 0, m.DeleteUserLabelErr
	}
	if m.Err != nil {
		return 0, m.Err
	}
	return m.DeleteUserLabelRows, nil
}

func (m *MockDB) CreateAlertChannel(_ context.Context, arg database.CreateAlertChannelParams) (database.AlertChannel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertChannelErr != nil {
		return database.AlertChannel{}, m.AlertChannelErr
	}
	ch := database.AlertChannel{
		ID:     newTestUUID(),
		Name:   arg.Name,
		Type:   arg.Type,
		Config: arg.Config,
	}
	m.AlertChannels[formatUUID(ch.ID)] = ch
	return ch, nil
}

func (m *MockDB) GetAlertChannel(_ context.Context, id pgtype.UUID) (database.AlertChannel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertChannelErr != nil {
		return database.AlertChannel{}, m.AlertChannelErr
	}
	ch, ok := m.AlertChannels[formatUUID(id)]
	if !ok {
		return database.AlertChannel{}, fmt.Errorf("alert channel not found")
	}
	return ch, nil
}

func (m *MockDB) ListAlertChannels(_ context.Context) ([]database.AlertChannel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertChannelErr != nil {
		return nil, m.AlertChannelErr
	}
	out := make([]database.AlertChannel, 0, len(m.AlertChannels))
	for _, ch := range m.AlertChannels {
		out = append(out, ch)
	}
	return out, nil
}

func (m *MockDB) UpdateAlertChannel(_ context.Context, arg database.UpdateAlertChannelParams) (database.AlertChannel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertChannelErr != nil {
		return database.AlertChannel{}, m.AlertChannelErr
	}
	key := formatUUID(arg.ID)
	ch, ok := m.AlertChannels[key]
	if !ok {
		return database.AlertChannel{}, fmt.Errorf("alert channel not found")
	}
	ch.Name = arg.Name
	ch.Type = arg.Type
	ch.Config = arg.Config
	m.AlertChannels[key] = ch
	return ch, nil
}

func (m *MockDB) DeleteAlertChannel(_ context.Context, id pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertChannelErr != nil {
		return m.AlertChannelErr
	}
	delete(m.AlertChannels, formatUUID(id))
	return nil
}

func (m *MockDB) CreateAlertRule(_ context.Context, arg database.CreateAlertRuleParams) (database.AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertRuleErr != nil {
		return database.AlertRule{}, m.AlertRuleErr
	}
	r := database.AlertRule{
		ID:              newTestUUID(),
		Name:            arg.Name,
		Enabled:         arg.Enabled,
		Scope:           arg.Scope,
		AgentID:         arg.AgentID,
		ConditionType:   arg.ConditionType,
		ConditionParams: arg.ConditionParams,
		CooldownSeconds: arg.CooldownSeconds,
	}
	m.AlertRules[formatUUID(r.ID)] = r
	return r, nil
}

func (m *MockDB) GetAlertRule(_ context.Context, id pgtype.UUID) (database.AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertRuleErr != nil {
		return database.AlertRule{}, m.AlertRuleErr
	}
	r, ok := m.AlertRules[formatUUID(id)]
	if !ok {
		return database.AlertRule{}, fmt.Errorf("alert rule not found")
	}
	return r, nil
}

func (m *MockDB) ListAlertRules(_ context.Context) ([]database.AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertRuleErr != nil {
		return nil, m.AlertRuleErr
	}
	out := make([]database.AlertRule, 0, len(m.AlertRules))
	for _, r := range m.AlertRules {
		out = append(out, r)
	}
	return out, nil
}

func (m *MockDB) ListEnabledAlertRules(_ context.Context) ([]database.AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertRuleErr != nil {
		return nil, m.AlertRuleErr
	}
	var out []database.AlertRule
	for _, r := range m.AlertRules {
		if r.Enabled {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *MockDB) UpdateAlertRule(_ context.Context, arg database.UpdateAlertRuleParams) (database.AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertRuleErr != nil {
		return database.AlertRule{}, m.AlertRuleErr
	}
	key := formatUUID(arg.ID)
	r, ok := m.AlertRules[key]
	if !ok {
		return database.AlertRule{}, fmt.Errorf("alert rule not found")
	}
	r.Name = arg.Name
	r.Enabled = arg.Enabled
	r.ConditionParams = arg.ConditionParams
	r.CooldownSeconds = arg.CooldownSeconds
	m.AlertRules[key] = r
	return r, nil
}

func (m *MockDB) DeleteAlertRule(_ context.Context, id pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertRuleErr != nil {
		return m.AlertRuleErr
	}
	delete(m.AlertRules, formatUUID(id))
	return nil
}

func (m *MockDB) SetAlertRuleEnabled(_ context.Context, arg database.SetAlertRuleEnabledParams) (database.AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertRuleErr != nil {
		return database.AlertRule{}, m.AlertRuleErr
	}
	key := formatUUID(arg.ID)
	r, ok := m.AlertRules[key]
	if !ok {
		return database.AlertRule{}, fmt.Errorf("alert rule not found")
	}
	r.Enabled = arg.Enabled
	m.AlertRules[key] = r
	return r, nil
}

func (m *MockDB) AddChannelToRule(_ context.Context, arg database.AddChannelToRuleParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := formatUUID(arg.RuleID)
	for _, id := range m.RuleChannels[key] {
		if id == arg.ChannelID {
			return nil // ON CONFLICT DO NOTHING
		}
	}
	m.RuleChannels[key] = append(m.RuleChannels[key], arg.ChannelID)
	return nil
}

func (m *MockDB) RemoveChannelFromRule(_ context.Context, arg database.RemoveChannelFromRuleParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := formatUUID(arg.RuleID)
	channels := m.RuleChannels[key]
	for i, id := range channels {
		if id == arg.ChannelID {
			m.RuleChannels[key] = append(channels[:i], channels[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *MockDB) ListChannelsForRule(_ context.Context, ruleID pgtype.UUID) ([]database.AlertChannel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []database.AlertChannel
	for _, chID := range m.RuleChannels[formatUUID(ruleID)] {
		if ch, ok := m.AlertChannels[formatUUID(chID)]; ok {
			out = append(out, ch)
		}
	}
	return out, nil
}

func (m *MockDB) DeleteChannelsForRule(_ context.Context, ruleID pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.RuleChannels, formatUUID(ruleID))
	return nil
}

func (m *MockDB) GetActiveEvent(_ context.Context, arg database.GetActiveEventParams) (database.AlertEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertEventErr != nil {
		return database.AlertEvent{}, m.AlertEventErr
	}
	for _, ev := range m.AlertEvents {
		if ev.RuleID == arg.RuleID && ev.AgentID == arg.AgentID && !ev.ResolvedAt.Valid {
			return ev, nil
		}
	}
	return database.AlertEvent{}, fmt.Errorf("no active event")
}

func (m *MockDB) GetLastEventForRule(_ context.Context, arg database.GetLastEventForRuleParams) (database.AlertEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertEventErr != nil {
		return database.AlertEvent{}, m.AlertEventErr
	}
	var latest database.AlertEvent
	found := false
	for _, ev := range m.AlertEvents {
		if ev.RuleID == arg.RuleID && ev.AgentID == arg.AgentID {
			if !found || ev.FiredAt.Time.After(latest.FiredAt.Time) {
				latest = ev
				found = true
			}
		}
	}
	if !found {
		return database.AlertEvent{}, fmt.Errorf("no event found")
	}
	return latest, nil
}

func (m *MockDB) CreateAlertEvent(_ context.Context, arg database.CreateAlertEventParams) (database.AlertEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertEventErr != nil {
		return database.AlertEvent{}, m.AlertEventErr
	}
	ev := database.AlertEvent{
		ID:                newTestUUID(),
		RuleID:            arg.RuleID,
		AgentID:           arg.AgentID,
		FiredAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ConditionSnapshot: arg.ConditionSnapshot,
		LastNotifiedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.AlertEvents[formatUUID(ev.ID)] = ev
	return ev, nil
}

func (m *MockDB) ResolveAlertEvent(_ context.Context, arg database.ResolveAlertEventParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertEventErr != nil {
		return m.AlertEventErr
	}
	for key, ev := range m.AlertEvents {
		if ev.RuleID == arg.RuleID && ev.AgentID == arg.AgentID && !ev.ResolvedAt.Valid {
			ev.ResolvedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			m.AlertEvents[key] = ev
			return nil
		}
	}
	return nil
}

func (m *MockDB) TouchAlertEventNotified(_ context.Context, id pgtype.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertEventErr != nil {
		return m.AlertEventErr
	}
	key := formatUUID(id)
	if ev, ok := m.AlertEvents[key]; ok {
		ev.LastNotifiedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		m.AlertEvents[key] = ev
	}
	return nil
}

func (m *MockDB) ListActiveAlertEvents(_ context.Context) ([]database.ListActiveAlertEventsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertEventErr != nil {
		return nil, m.AlertEventErr
	}
	var out []database.ListActiveAlertEventsRow
	for _, ev := range m.AlertEvents {
		if !ev.ResolvedAt.Valid {
			r, ok := m.AlertRules[formatUUID(ev.RuleID)]
			if !ok {
				continue
			}
			out = append(out, database.ListActiveAlertEventsRow{
				ID:                ev.ID,
				RuleID:            ev.RuleID,
				AgentID:           ev.AgentID,
				FiredAt:           ev.FiredAt,
				ResolvedAt:        ev.ResolvedAt,
				LastNotifiedAt:    ev.LastNotifiedAt,
				ConditionSnapshot: ev.ConditionSnapshot,
				RuleName:          r.Name,
				ConditionType:     r.ConditionType,
			})
		}
	}
	return out, nil
}

func (m *MockDB) ListAlertEventHistory(_ context.Context, arg database.ListAlertEventHistoryParams) ([]database.ListAlertEventHistoryRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertEventErr != nil {
		return nil, m.AlertEventErr
	}
	return []database.ListAlertEventHistoryRow{}, nil
}

func (m *MockDB) ListAlertEventsByAgent(_ context.Context, arg database.ListAlertEventsByAgentParams) ([]database.ListAlertEventsByAgentRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertEventErr != nil {
		return nil, m.AlertEventErr
	}
	return []database.ListAlertEventsByAgentRow{}, nil
}

func (m *MockDB) GetDiskTrend(_ context.Context, _ database.GetDiskTrendParams) ([]database.GetDiskTrendRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.DiskTrendErr != nil {
		return nil, m.DiskTrendErr
	}
	return m.DiskTrendRows, nil
}

func (m *MockDB) ListAllActiveEvents(_ context.Context) ([]database.AlertEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertEventErr != nil {
		return nil, m.AlertEventErr
	}
	var out []database.AlertEvent
	for _, ev := range m.AlertEvents {
		if !ev.ResolvedAt.Valid {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (m *MockDB) ListLastEventPerRuleAgent(_ context.Context) ([]database.AlertEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.AlertEventErr != nil {
		return nil, m.AlertEventErr
	}
	// Most recent event per (rule_id, agent_id).
	latest := make(map[string]database.AlertEvent)
	for _, ev := range m.AlertEvents {
		key := formatUUID(ev.RuleID) + ":" + formatUUID(ev.AgentID)
		if cur, ok := latest[key]; !ok || ev.FiredAt.Time.After(cur.FiredAt.Time) {
			latest[key] = ev
		}
	}
	out := make([]database.AlertEvent, 0, len(latest))
	for _, ev := range latest {
		out = append(out, ev)
	}
	return out, nil
}

func (m *MockDB) GetAllServices(_ context.Context) ([]database.CurrentService, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return m.AllServices, nil
}

func (m *MockDB) GetSMTPConfig(_ context.Context) (database.SmtpConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SMTPConfigErr != nil {
		return database.SmtpConfig{}, m.SMTPConfigErr
	}
	if m.SMTPConfig == nil {
		return database.SmtpConfig{}, pgx.ErrNoRows
	}
	return *m.SMTPConfig, nil
}

func (m *MockDB) UpsertSMTPConfig(_ context.Context, arg database.UpsertSMTPConfigParams) (database.SmtpConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SMTPConfigErr != nil {
		return database.SmtpConfig{}, m.SMTPConfigErr
	}
	cfg := database.SmtpConfig{
		ID:                true,
		Enabled:           arg.Enabled,
		Host:              arg.Host,
		Port:              arg.Port,
		Username:          arg.Username,
		PasswordEncrypted: arg.PasswordEncrypted,
		FromAddress:       arg.FromAddress,
		TlsMode:           arg.TlsMode,
		UpdatedAt:         pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.SMTPConfig = &cfg
	return cfg, nil
}

func (m *MockDB) ListAllAgentLabels(_ context.Context) ([]database.ListAllAgentLabelsRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	if m.Err != nil {
		return nil, m.Err
	}
	return m.ListAllAgentLabelsReturn, nil
}
