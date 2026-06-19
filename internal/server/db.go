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
	UpdateAgentVersion(ctx context.Context, arg database.UpdateAgentVersionParams) error

	// Auth
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
	GetAgent(ctx context.Context, id pgtype.UUID) (database.GetAgentRow, error)
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
	GetCPUBucketed(ctx context.Context, arg database.GetCPUBucketedParams) ([]database.GetCPUBucketedRow, error)
	GetMemoryBucketed(ctx context.Context, arg database.GetMemoryBucketedParams) ([]database.GetMemoryBucketedRow, error)
	GetDiskBucketed(ctx context.Context, arg database.GetDiskBucketedParams) ([]database.GetDiskBucketedRow, error)
	GetDiskIOBucketed(ctx context.Context, arg database.GetDiskIOBucketedParams) ([]database.GetDiskIOBucketedRow, error)
	GetNetworkBucketed(ctx context.Context, arg database.GetNetworkBucketedParams) ([]database.GetNetworkBucketedRow, error)
	GetTemperatureBucketed(ctx context.Context, arg database.GetTemperatureBucketedParams) ([]database.GetTemperatureBucketedRow, error)
	GetSystemBucketed(ctx context.Context, arg database.GetSystemBucketedParams) ([]database.GetSystemBucketedRow, error)
	GetContainerBucketed(ctx context.Context, arg database.GetContainerBucketedParams) ([]database.GetContainerBucketedRow, error)
	GetWifiBucketed(ctx context.Context, arg database.GetWifiBucketedParams) ([]database.GetWifiBucketedRow, error)
	GetPiBucketed(ctx context.Context, arg database.GetPiBucketedParams) ([]database.GetPiBucketedRow, error)
	GetFleetHeatmap(ctx context.Context, arg database.GetFleetHeatmapParams) ([]database.GetFleetHeatmapRow, error)

	// Read API - current state
	GetProcessesByCPU(ctx context.Context, args database.GetProcessesByCPUParams) ([]database.CurrentProcess, error)
	GetProcessesByMemory(ctx context.Context, args database.GetProcessesByMemoryParams) ([]database.CurrentProcess, error)
	GetServices(ctx context.Context, id pgtype.UUID) ([]database.CurrentService, error)
	GetApplications(ctx context.Context, id pgtype.UUID) ([]database.CurrentApplication, error)
	GetUpdates(ctx context.Context, id pgtype.UUID) (database.CurrentUpdate, error)
	GetLatestSystem(ctx context.Context, id pgtype.UUID) (database.MetricsSystem, error)

	// Read API - Sparklines
	GetRecentCPU(ctx context.Context) ([]database.GetRecentCPURow, error)
	GetRecentMemory(ctx context.Context) ([]database.GetRecentMemoryRow, error)
	GetRecentDiskMax(ctx context.Context) ([]database.GetRecentDiskMaxRow, error)
	GetFleetSparkCPU(ctx context.Context, args database.GetFleetSparkCPUParams) ([]database.GetFleetSparkCPURow, error)
	GetFleetSparkMemory(ctx context.Context, args database.GetFleetSparkMemoryParams) ([]database.GetFleetSparkMemoryRow, error)
	GetFleetSparkDisk(ctx context.Context, args database.GetFleetSparkDiskParams) ([]database.GetFleetSparkDiskRow, error)

	// SHA-256 migration
	GetAgentSecretSHA256(ctx context.Context, id pgtype.UUID) ([]byte, error)
	SetAgentSecretSHA256(ctx context.Context, arg database.SetAgentSecretSHA256Params) error
	TouchLastSeenIfStale(ctx context.Context, arg database.TouchLastSeenIfStaleParams) error

	// Interface
	GetAgentConfig(ctx context.Context, id pgtype.UUID) ([]database.AgentConfig, error)
	GetAgentConfigByKey(ctx context.Context, arg database.GetAgentConfigByKeyParams) (database.AgentConfig, error)
	SetAgentConfig(ctx context.Context, arg database.SetAgentConfigParams) error
	DeleteAgentConfig(ctx context.Context, arg database.DeleteAgentConfigParams) error
	DeleteAllAgentConfig(ctx context.Context, id pgtype.UUID) error

	// Users
	ListUsers(ctx context.Context) ([]database.ListUsersRow, error)
	ListUsersWithLastLogin(ctx context.Context) ([]database.ListUsersWithLastLoginRow, error)
	CreateUser(ctx context.Context, arg database.CreateUserParams) error
	UpsertSuperadmin(ctx context.Context, arg database.UpsertSuperadminParams) error
	GetUserByID(ctx context.Context, id pgtype.UUID) (database.GetUserByIDRow, error)
	SuperAdminCount(ctx context.Context) (int64, error)
	DeleteUser(ctx context.Context, id pgtype.UUID) error
	UpdateUserRole(ctx context.Context, arg database.UpdateUserRoleParams) error
	GetUserConfig(ctx context.Context, id pgtype.UUID) ([]database.GetUserConfigRow, error)
	SetUserConfig(ctx context.Context, arg database.SetUserConfigParams) error
	DeleteUserConfig(ctx context.Context, arg database.DeleteUserConfigParams) error

	// Labels
	ReplaceAutoLabels(ctx context.Context, arg database.ReplaceAutoLabelsParams) error
	UpsertAutoLabel(ctx context.Context, arg database.UpsertAutoLabelParams) error
	ListAgentLabels(ctx context.Context, id pgtype.UUID) ([]database.ListAgentLabelsRow, error)
	ListLabelKeys(ctx context.Context) ([]database.ListLabelKeysRow, error)
	ListLabelValuesForKey(ctx context.Context, key string) ([]string, error)
	UpsertUserLabel(ctx context.Context, arg database.UpsertUserLabelParams) (database.AgentLabel, error)
	GetAgentLabel(ctx context.Context, arg database.GetAgentLabelParams) (database.GetAgentLabelRow, error)
	DeleteUserLabel(ctx context.Context, arg database.DeleteUserLabelParams) (int64, error)

	PurgeOfflineAgents(ctx context.Context) (int64, error)

	// Alert channels
	CreateAlertChannel(ctx context.Context, arg database.CreateAlertChannelParams) (database.AlertChannel, error)
	GetAlertChannel(ctx context.Context, id pgtype.UUID) (database.AlertChannel, error)
	ListAlertChannels(ctx context.Context) ([]database.AlertChannel, error)
	UpdateAlertChannel(ctx context.Context, arg database.UpdateAlertChannelParams) (database.AlertChannel, error)
	DeleteAlertChannel(ctx context.Context, id pgtype.UUID) error

	// Alert rules
	CreateAlertRule(ctx context.Context, arg database.CreateAlertRuleParams) (database.AlertRule, error)
	GetAlertRule(ctx context.Context, id pgtype.UUID) (database.AlertRule, error)
	ListAlertRules(ctx context.Context) ([]database.AlertRule, error)
	ListEnabledAlertRules(ctx context.Context) ([]database.AlertRule, error)
	UpdateAlertRule(ctx context.Context, arg database.UpdateAlertRuleParams) (database.AlertRule, error)
	DeleteAlertRule(ctx context.Context, id pgtype.UUID) error
	SetAlertRuleEnabled(ctx context.Context, arg database.SetAlertRuleEnabledParams) (database.AlertRule, error)

	// Alert rule channels
	AddChannelToRule(ctx context.Context, arg database.AddChannelToRuleParams) error
	RemoveChannelFromRule(ctx context.Context, arg database.RemoveChannelFromRuleParams) error
	ListChannelsForRule(ctx context.Context, ruleID pgtype.UUID) ([]database.AlertChannel, error)
	DeleteChannelsForRule(ctx context.Context, ruleID pgtype.UUID) error

	// Alert events
	GetActiveEvent(ctx context.Context, arg database.GetActiveEventParams) (database.AlertEvent, error)
	GetLastEventForRule(ctx context.Context, arg database.GetLastEventForRuleParams) (database.AlertEvent, error)
	CreateAlertEvent(ctx context.Context, arg database.CreateAlertEventParams) (database.AlertEvent, error)
	ResolveAlertEvent(ctx context.Context, arg database.ResolveAlertEventParams) error
	TouchAlertEventNotified(ctx context.Context, id pgtype.UUID) error
	ListActiveAlertEvents(ctx context.Context) ([]database.ListActiveAlertEventsRow, error)
	ListAlertEventHistory(ctx context.Context, arg database.ListAlertEventHistoryParams) ([]database.ListAlertEventHistoryRow, error)
	ListAlertEventsByAgent(ctx context.Context, arg database.ListAlertEventsByAgentParams) ([]database.ListAlertEventsByAgentRow, error)

	// Bulk preload (alert evaluator)
	ListAllActiveEvents(ctx context.Context) ([]database.AlertEvent, error)
	ListLastEventPerRuleAgent(ctx context.Context) ([]database.AlertEvent, error)
	GetAllServices(ctx context.Context) ([]database.CurrentService, error)

	// Disk trend (for disk_prediction evaluator)
	GetDiskTrend(ctx context.Context, arg database.GetDiskTrendParams) ([]database.GetDiskTrendRow, error)

	// SMTP Config
	GetSMTPConfig(ctx context.Context) (database.SmtpConfig, error)
	UpsertSMTPConfig(ctx context.Context, arg database.UpsertSMTPConfigParams) (database.SmtpConfig, error)
}

// Compile-time check that *database.Queries satisfies the DB interface.
var _ DB = (*database.Queries)(nil)
