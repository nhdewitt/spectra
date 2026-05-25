package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/nhdewitt/spectra/internal/logging"
	"github.com/nhdewitt/spectra/internal/version"
	"golang.org/x/net/netutil"
)

type Config struct {
	Port           int
	ExternalURL    string
	CommandTimeout time.Duration
	ReleasesDir    string // path to pre-built agent binaries
	MaxConnections uint
	LogFile        string // path to JSON log file
	LogLevel       string // "debug", "info", "warn", "error"
}

type Server struct {
	Config       Config
	CmdQueue     *CommandQueue
	Tokens       *TokenStore
	DB           DB
	Router       *http.ServeMux
	Logger       *logging.Logger
	LoginTracker *loginTracker
	Limiters     *tieredLimiters
	Releases     *releaseManifest
	httpServer   *http.Server
	Commands     *commandResultStore

	done chan struct{}
}

func New(cfg Config, db DB) *Server {
	if cfg.CommandTimeout == 0 {
		cfg.CommandTimeout = 30 * time.Second
	}

	logCfg := logging.DefaultServerConfig()
	if cfg.LogFile != "" {
		logCfg.FilePath = cfg.LogFile
	}
	if cfg.LogLevel != "" {
		logCfg.ConsoleLevel = logging.ParseLevel(cfg.LogLevel)
	}

	logger := logging.New(logCfg)

	s := &Server{
		Config:       cfg,
		CmdQueue:     NewCommandQueue(),
		Tokens:       NewTokenStore(),
		DB:           db,
		Router:       http.NewServeMux(),
		Logger:       logger,
		LoginTracker: newLoginTracker(),
		Limiters:     newTieredLimiters(),
		Releases:     newReleaseManifest(cfg.ReleasesDir),
		Commands:     newCommandResultStore(10 * time.Minute),
		done:         make(chan struct{}),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// Auth (public, anonymous rate limit)
	s.Router.HandleFunc("POST /api/v1/auth/login", s.rateLimit(s.handleLogin))
	s.Router.HandleFunc("POST /api/v1/auth/logout", s.rateLimit(s.handleLogout))
	s.Router.HandleFunc("GET /api/v1/auth/me", s.rateLimit(s.requireUserAuth(s.handleMe)))

	// Agent (agent auth, agent rate limit)
	s.Router.HandleFunc("POST /api/v1/agent/register", s.rateLimit(s.handleAgentRegister))
	s.Router.HandleFunc("POST /api/v1/agent/metrics", s.rateLimitAgent(s.requireAgentAuth(s.handleMetrics)))
	s.Router.HandleFunc("GET /api/v1/agent/command", s.rateLimitAgent(s.requireAgentAuth(s.handleAgentCommand)))
	s.Router.HandleFunc("POST /api/v1/agent/command/result", s.rateLimitAgent(s.requireAgentAuth(s.handleCommandResult)))
	s.Router.HandleFunc("GET /api/v1/agent/config", s.requireAgentAuth(s.handleGetAgentSelfConfig))

	// Dashboard (user auth, authed rate limit)
	s.Router.HandleFunc("GET /api/v1/overview", s.requireUserAuth(s.rateLimitAuthed(s.handleOverview)))
	s.Router.HandleFunc("GET /api/v1/overview/sparklines", s.requireUserAuth(s.rateLimitAuthed(s.handleGetSparklines)))
	s.Router.HandleFunc("GET /api/v1/overview/fleet/chart", s.requireUserAuth(s.rateLimitAuthed(s.handleFleetChart)))
	s.Router.HandleFunc("GET /api/v1/agents", s.requireUserAuth(s.rateLimitAuthed(s.handleListAgents)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}", s.requireUserAuth(s.rateLimitAuthed(s.handleGetAgent)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/config", s.requireUserAuth(s.rateLimitAuthed(s.handleGetAgentConfig)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/cpu", s.requireUserAuth(s.rateLimitAuthed(s.handleGetCPU)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/memory", s.requireUserAuth(s.rateLimitAuthed(s.handleGetMemory)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/disk", s.requireUserAuth(s.rateLimitAuthed(s.handleGetDisk)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/diskio", s.requireUserAuth(s.rateLimitAuthed(s.handleGetDiskIO)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/network", s.requireUserAuth(s.rateLimitAuthed(s.handleGetNetwork)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/temperature", s.requireUserAuth(s.rateLimitAuthed(s.handleGetTemperature)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/system", s.requireUserAuth(s.rateLimitAuthed(s.handleGetSystem)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/containers", s.requireUserAuth(s.rateLimitAuthed(s.handleGetContainers)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/wifi", s.requireUserAuth(s.rateLimitAuthed(s.handleGetWifi)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/pi", s.requireUserAuth(s.rateLimitAuthed(s.handleGetPi)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/processes", s.requireUserAuth(s.rateLimitAuthed(s.handleGetProcesses)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/services", s.requireUserAuth(s.rateLimitAuthed(s.handleGetServices)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/applications", s.requireUserAuth(s.rateLimitAuthed(s.handleGetApplications)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/updates", s.requireUserAuth(s.rateLimitAuthed(s.handleGetUpdates)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/system/latest", s.requireUserAuth(s.rateLimitAuthed(s.handleGetLatestSystem)))
	s.Router.HandleFunc("GET /api/v1/admin/commands/{id}", s.requireUserAuth(s.rateLimitAuthed(s.handleGetCommandResult)))
	s.Router.HandleFunc("GET /api/v1/overview/heatmap", s.requireUserAuth(s.rateLimitAuthed(s.handleFleetHeatmap)))

	// Provision (user auth, authed rate limit)
	s.Router.HandleFunc("GET /api/v1/admin/platforms", s.requireUserAuth(s.rateLimitAuthed(s.handleListPlatforms)))
	s.Router.HandleFunc("POST /api/v1/admin/provision/config", s.requireUserAuth(s.rateLimitAuthed(s.handleDownloadConfig)))
	s.Router.HandleFunc("GET /api/v1/admin/releases/{filename}", s.tokenOrAuth(s.handleDownloadRelease))

	// Upgrade/uninstall instructions
	s.Router.HandleFunc("GET /api/v1/agents/{id}/upgrade-instructions", s.requireUserAuth(s.rateLimitAuthed(s.handleUpgradeInstructions)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/uninstall-instructions", s.requireUserAuth(s.rateLimitAuthed(s.handleUninstallInstructions)))

	s.Router.HandleFunc("GET /api/v1/version", s.rateLimit(s.handleVersion))

	// User management
	s.Router.HandleFunc("GET /api/v1/admin/users", s.requireUserAuth(s.rateLimitAuthed(s.handleListUsers)))
	s.Router.HandleFunc("POST /api/v1/admin/users", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handleCreateUser))))
	s.Router.HandleFunc("DELETE /api/v1/admin/users/{id}", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handleDeleteUser))))
	s.Router.HandleFunc("PUT /api/v1/admin/users/{id}/role", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleSuperAdmin)(s.handleUpdateUserRole))))

	// Operational write endpoints (admin+)
	s.Router.HandleFunc("DELETE /api/v1/agents/{id}", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handleDeleteAgent))))
	s.Router.HandleFunc("PUT /api/v1/agents/{id}/config", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handleSetAgentConfig))))
	s.Router.HandleFunc("DELETE /api/v1/agents/{id}/config", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handleDeleteAgentConfig))))
	s.Router.HandleFunc("POST /api/v1/admin/logs", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handleAdminTriggerLogs))))
	s.Router.HandleFunc("POST /api/v1/admin/disk", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handleAdminTriggerDisk))))
	s.Router.HandleFunc("POST /api/v1/admin/network", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handleAdminTriggerNetwork))))
	s.Router.HandleFunc("POST /api/v1/admin/tokens", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handleGenerateToken))))
	s.Router.HandleFunc("POST /api/v1/admin/provision", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handleProvision))))
	s.Router.HandleFunc("POST /api/v1/admin/agents/purge", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handlePurgeOfflineAgents))))
	s.Router.HandleFunc("POST /api/v1/admin/tokens/revoke", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handleRevokeAllTokens))))
	s.Router.HandleFunc("POST /api/v1/admin/update", s.requireUserAuth(s.rateLimitAuthed(requireRole(RoleAdmin)(s.handlePushUpdate))))

	// User config (any authenticated user)
	s.Router.HandleFunc("GET /api/v1/user/config", s.requireUserAuth(s.rateLimitAuthed(s.handleGetUserConfig)))
	s.Router.HandleFunc("PUT /api/v1/user/config", s.requireUserAuth(s.rateLimitAuthed(s.handleSetUserConfig)))
	s.Router.HandleFunc("DELETE /api/v1/user/config", s.requireUserAuth(s.rateLimitAuthed(s.handleDeleteUserConfig)))

	// API catch-all: reject unmatched /api/ routes before SPA fallback
	s.Router.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	// Embedded frontend (SPA fallback)
	s.Router.Handle("/", spaHandler())
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.Config.Port)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.requestLogger(s.Router),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      40 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	ln = netutil.LimitListener(ln, int(s.Config.MaxConnections))

	s.Logger.Info("server started", "addr", addr, "version", version.Full())
	return s.httpServer.Serve(ln)
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.Logger.Info("server shutting down")
	close(s.done)
	s.Limiters.Stop()
	s.Commands.Stop()
	err := s.httpServer.Shutdown(ctx)
	s.Logger.Close() // flush
	return err
}
