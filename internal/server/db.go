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
}

// Compile-time check that *database.Queries satisfies the DB interface.
var _ DB = (*database.Queries)(nil)
