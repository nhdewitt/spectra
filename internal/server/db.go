package server

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

// DB defines the database operations the server depends on.
// Implemented by *database.Queries for production and MockDB for tests.
type DB interface {
	// Agent management
	RegisterAgent(ctx context.Context, arg database.RegisterAgentParams) error
	GetAgentSecret(ctx context.Context, id pgtype.UUID) (string, error)
	TouchLastSeen(ctx context.Context, id pgtype.UUID) error
	AgentExists(ctx context.Context, id pgtype.UUID) (bool, error)
	ListAgents(ctx context.Context) ([]database.ListAgentsRow, error)

	// Auth
	CreateUser(ctx context.Context, arg database.CreateUserParams) error
	GetUserByUsername(ctx context.Context, username string) (database.GetUserByUsernameRow, error)
	UserCount(ctx context.Context) (int64, error)
	CreateSession(ctx context.Context, arg database.CreateSessionParams) error
	GetSession(ctx context.Context, token string) (database.GetSessionRow, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteExpiredSessions(ctx context.Context) error
	DeleteUserSessions(ctx context.Context, userID pgtype.UUID) error

	// Metric inserts
	InsertCPU(ctx context.Context, arg database.InsertCPUParams) error
	InsertMemory(ctx context.Context, arg database.InsertMemoryParams) error
	InsertDisk(ctx context.Context, arg database.InsertDiskParams) error
	InsertDiskIO(ctx context.Context, arg database.InsertDiskIOParams) error
	InsertNetwork(ctx context.Context, arg database.InsertNetworkParams) error
	InsertTemperature(ctx context.Context, arg database.InsertTemperatureParams) error
	InsertWifi(ctx context.Context, arg database.InsertWifiParams) error
	InsertSystem(ctx context.Context, arg database.InsertSystemParams) error
	InsertContainer(ctx context.Context, arg database.InsertContainerParams) error
	InsertPi(ctx context.Context, arg database.InsertPiParams) error

	// Current-state upserts
	UpsertProcess(ctx context.Context, arg database.UpsertProcessParams) error
	DeleteStaleProcesses(ctx context.Context, arg database.DeleteStaleProcessesParams) error
	UpsertService(ctx context.Context, arg database.UpsertServiceParams) error
	UpsertApplication(ctx context.Context, arg database.UpsertApplicationParams) error
	UpsertUpdates(ctx context.Context, arg database.UpsertUpdatesParams) error
	UpsertCurrentCPU(ctx context.Context, arg database.UpsertCurrentCPUParams) error
	UpsertCurrentMemory(ctx context.Context, arg database.UpsertCurrentMemoryParams) error
	UpsertCurrentDiskMax(ctx context.Context, id pgtype.UUID) error
	UpsertCurrentNetwork(ctx context.Context, id pgtype.UUID) error
	UpsertCurrentTemperature(ctx context.Context, id pgtype.UUID) error
	UpsertCurrentSystem(ctx context.Context, arg database.UpsertCurrentSystemParams) error
	UpsertCurrentReboot(ctx context.Context, arg database.UpsertCurrentRebootParams) error

	// Read API - overview
	GetOverview(ctx context.Context) ([]database.GetOverviewRow, error)

	// Read API - agent management
	GetAgent(ctx context.Context, id pgtype.UUID) (database.Agent, error)
	DeleteAgent(ctx context.Context, id pgtype.UUID) error

	// Read API - time-series metrics (timestamp)
	GetCPURange(ctx context.Context, arg database.GetCPURangeParams) ([]database.MetricsCpu, error)
	GetMemoryRange(ctx context.Context, arg database.GetMemoryRangeParams) ([]database.MetricsMemory, error)
	GetDiskRange(ctx context.Context, arg database.GetDiskRangeParams) ([]database.MetricsDisk, error)
	GetDiskIORange(ctx context.Context, arg database.GetDiskIORangeParams) ([]database.MetricsDiskIo, error)
	GetNetworkRange(ctx context.Context, arg database.GetNetworkRangeParams) ([]database.MetricsNetwork, error)
	GetTemperatureRange(ctx context.Context, arg database.GetTemperatureRangeParams) ([]database.MetricsTemperature, error)
	GetSystemRange(ctx context.Context, arg database.GetSystemRangeParams) ([]database.MetricsSystem, error)
	GetContainerRange(ctx context.Context, arg database.GetContainerRangeParams) ([]database.MetricsContainer, error)
	GetWifiRange(ctx context.Context, arg database.GetWifiRangeParams) ([]database.MetricsWifi, error)
	GetPiRange(ctx context.Context, arg database.GetPiRangeParams) ([]database.GetPiRangeRow, error)

	// Read API - current state
	GetProcessesByCPU(ctx context.Context, args database.GetProcessesByCPUParams) ([]database.CurrentProcess, error)
	GetProcessesByMemory(ctx context.Context, args database.GetProcessesByMemoryParams) ([]database.CurrentProcess, error)
	GetServices(ctx context.Context, id pgtype.UUID) ([]database.CurrentService, error)
	GetApplications(ctx context.Context, id pgtype.UUID) ([]database.CurrentApplication, error)
	GetUpdates(ctx context.Context, id pgtype.UUID) (database.CurrentUpdate, error)

	// Read API - Sparklines
	GetRecentCPU(ctx context.Context) ([]database.GetRecentCPURow, error)
	GetRecentMemory(ctx context.Context) ([]database.GetRecentMemoryRow, error)
	GetRecentDiskMax(ctx context.Context) ([]database.GetRecentDiskMaxRow, error)
}

// Compile-time check that *database.Queries satisfies the DB interface.
var _ DB = (*database.Queries)(nil)
