package server

import (
	"fmt"
	"net/http"
	"time"
)

type Config struct {
	Port           int
	CommandTimeout time.Duration
}

type Server struct {
	Config       Config
	Store        *AgentStore
	Tokens       *TokenStore
	DB           DB
	Router       *http.ServeMux
	LoginTracker *loginTracker
	Limiter      *rateLimiter
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
		Limiter:      newRateLimiter(requestsPerSecond, burst),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// Auth (public, rate limited)
	s.Router.HandleFunc("POST /api/v1/auth/login", s.rateLimit(s.handleLogin))
	s.Router.HandleFunc("POST /api/v1/auth/logout", s.rateLimit(s.handleLogout))
	s.Router.HandleFunc("GET /api/v1/auth/me", s.rateLimit(s.requireUserAuth(s.handleMe)))

	// Agent (agent auth, rate limited)
	s.Router.HandleFunc("POST /api/v1/agent/register", s.rateLimit(s.handleAgentRegister))
	s.Router.HandleFunc("POST /api/v1/agent/metrics", s.rateLimit(s.requireAgentAuth(s.handleMetrics)))
	s.Router.HandleFunc("GET /api/v1/agent/command", s.rateLimit(s.requireAgentAuth(s.handleAgentCommand)))
	s.Router.HandleFunc("POST /api/v1/agent/command/result", s.rateLimit(s.requireAgentAuth(s.handleCommandResult)))

	// Admin (user auth, rate limited)
	s.Router.HandleFunc("POST /api/v1/admin/logs", s.rateLimit(s.requireUserAuth(s.handleAdminTriggerLogs)))
	s.Router.HandleFunc("POST /api/v1/admin/disk", s.rateLimit(s.requireUserAuth(s.handleAdminTriggerDisk)))
	s.Router.HandleFunc("POST /api/v1/admin/network", s.rateLimit(s.requireUserAuth(s.handleAdminTriggerNetwork)))
	s.Router.HandleFunc("POST /api/v1/admin/tokens", s.rateLimit(s.requireUserAuth(s.handleGenerateToken)))

	// Dashboard (user auth, rate limited)
	s.Router.HandleFunc("GET /api/v1/overview", s.rateLimit(s.requireUserAuth(s.handleOverview)))
	s.Router.HandleFunc("GET /api/v1/agents", s.rateLimit(s.requireUserAuth(s.handleListAgents)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}", s.rateLimit(s.requireUserAuth(s.handleGetAgent)))
	s.Router.HandleFunc("DELETE /api/v1/agents/{id}", s.rateLimit(s.requireUserAuth(s.handleDeleteAgent)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/cpu", s.rateLimit(s.requireUserAuth(s.handleGetCPU)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/memory", s.rateLimit(s.requireUserAuth(s.handleGetMemory)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/disk", s.rateLimit(s.requireUserAuth(s.handleGetDisk)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/diskio", s.rateLimit(s.requireUserAuth(s.handleGetDiskIO)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/network", s.rateLimit(s.requireUserAuth(s.handleGetNetwork)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/temperature", s.rateLimit(s.requireUserAuth(s.handleGetTemperature)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/system", s.rateLimit(s.requireUserAuth(s.handleGetSystem)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/containers", s.rateLimit(s.requireUserAuth(s.handleGetContainers)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/wifi", s.rateLimit(s.requireUserAuth(s.handleGetWifi)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/pi", s.rateLimit(s.requireUserAuth(s.handleGetPi)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/processes", s.rateLimit(s.requireUserAuth(s.handleGetProcesses)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/services", s.rateLimit(s.requireUserAuth(s.handleGetServices)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/applications", s.rateLimit(s.requireUserAuth(s.handleGetApplications)))
	s.Router.HandleFunc("GET /api/v1/agents/{id}/updates", s.rateLimit(s.requireUserAuth(s.handleGetUpdates)))
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.Config.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.Router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 40 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	fmt.Printf("Spectra Server listening on %s...\n", addr)
	return srv.ListenAndServe()
}
