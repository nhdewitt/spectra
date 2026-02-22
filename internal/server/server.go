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
	Config Config
	Store  *AgentStore
	Tokens *TokenStore
	DB     DB
	Router *http.ServeMux
}

func New(cfg Config, db DB) *Server {
	if cfg.CommandTimeout == 0 {
		cfg.CommandTimeout = 30 * time.Second
	}

	s := &Server{
		Config: cfg,
		Store:  NewAgentStore(),
		Tokens: NewTokenStore(),
		DB:     db,
		Router: http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.Router.HandleFunc("POST /api/v1/agent/metrics", s.requireAgentAuth(s.handleMetrics))
	s.Router.HandleFunc("GET /api/v1/agent/command", s.requireAgentAuth(s.handleAgentCommand))
	s.Router.HandleFunc("POST /api/v1/agent/command/result", s.requireAgentAuth(s.handleCommandResult))
	s.Router.HandleFunc("POST /api/v1/agent/register", s.handleAgentRegister)
	s.Router.HandleFunc("POST /api/v1/admin/logs", s.handleAdminTriggerLogs)
	s.Router.HandleFunc("POST /api/v1/admin/disk", s.handleAdminTriggerDisk)
	s.Router.HandleFunc("POST /api/v1/admin/network", s.handleAdminTriggerNetwork)
	s.Router.HandleFunc("POST /api/v1/admin/tokens", s.handleGenerateToken)
	s.Router.HandleFunc("GET /api/v1/overview", s.handleOverview)
	s.Router.HandleFunc("GET /api/v1/agents", s.handleListAgents)
	s.Router.HandleFunc("GET /api/v1/agents/{id}", s.handleGetAgent)
	s.Router.HandleFunc("DELETE /api/v1/agents/{id}", s.handleDeleteAgent)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/cpu", s.handleGetCPU)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/memory", s.handleGetMemory)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/disk", s.handleGetDisk)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/diskio", s.handleGetDiskIO)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/network", s.handleGetNetwork)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/temperature", s.handleGetTemperature)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/system", s.handleGetSystem)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/containers", s.handleGetContainers)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/wifi", s.handleGetWifi)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/pi", s.handleGetPi)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/processes", s.handleGetProcesses)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/services", s.handleGetServices)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/applications", s.handleGetApplications)
	s.Router.HandleFunc("GET /api/v1/agents/{id}/updates", s.handleGetUpdates)
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
