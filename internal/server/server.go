package server

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Config struct {
	Port           int
	CommandTimeout time.Duration
	ReleasesDir    string // path to pre-built agent binaries
}

type Server struct {
	Config       Config
	Store        *AgentStore
	Tokens       *TokenStore
	DB           DB
	Router       *http.ServeMux
	LoginTracker *loginTracker
	Limiters     *tieredLimiters
	Releases     *releaseManifest
	httpServer   *http.Server
}

func New(cfg Config, db DB) *Server {
	if cfg.CommandTimeout == 0 {
		cfg.CommandTimeout = 30 * time.Second
	}

	s := &Server{
		Config:       cfg,
		Store:        NewAgentStore(),
		Tokens:       NewTokenStore(),
		DB:           db,
		Router:       http.NewServeMux(),
		LoginTracker: newLoginTracker(),
		Limiters:     newTieredLimiters(),
		Releases:     newReleaseManifest(cfg.ReleasesDir),
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

	// Admin (user auth, authed rate limit)
	s.Router.HandleFunc("POST /api/v1/admin/logs", s.requireUserAuth(s.rateLimitAuthed(s.handleAdminTriggerLogs)))
	s.Router.HandleFunc("POST /api/v1/admin/disk", s.requireUserAuth(s.rateLimitAuthed(s.handleAdminTriggerDisk)))
	s.Router.HandleFunc("POST /api/v1/admin/network", s.requireUserAuth(s.rateLimitAuthed(s.handleAdminTriggerNetwork)))
	s.Router.HandleFunc("POST /api/v1/admin/tokens", s.requireUserAuth(s.rateLimitAuthed(s.handleGenerateToken)))

	// Dashboard (user auth, authed rate limit)
	s.Router.HandleFunc("GET /api/v1/overview", s.requireUserAuth(s.rateLimitAuthed(s.handleOverview)))
	s.Router.HandleFunc("GET /api/v1/overview/sparklines", s.requireUserAuth(s.rateLimitAuthed(s.handleGetSparklines)))
	s.Router.HandleFunc("GET /api/v1/agents", s.requireUserAuth(s.rateLimitAuthed(s.handleListAgents)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}", s.requireUserAuth(s.rateLimitAuthed(s.handleGetAgent)))
	s.Router.HandleFunc("DELETE /api/v1/agents/{id}", s.requireUserAuth(s.rateLimitAuthed(s.handleDeleteAgent)))
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

	// Provision (user auth, authed rate limit)
	s.Router.HandleFunc("GET /api/v1/admin/platforms", s.requireUserAuth(s.rateLimitAuthed(s.handleListPlatforms)))
	s.Router.HandleFunc("POST /api/v1/admin/provision", s.requireUserAuth(s.rateLimitAuthed(s.handleProvision)))
	s.Router.HandleFunc("POST /api/v1/admin/provision/config", s.requireUserAuth(s.rateLimitAuthed(s.handleDownloadConfig)))
	s.Router.HandleFunc("GET /api/v1/admin/releases/{filename}", s.requireUserAuth(s.rateLimitAuthed(s.handleDownloadRelease)))
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.Config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.Router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 40 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	fmt.Printf("Spectra Server listening on %s...\n", addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.Limiters.Stop()
	return s.httpServer.Shutdown(ctx)
}
