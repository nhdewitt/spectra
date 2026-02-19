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
	s.Router.HandleFunc("/api/v1/agent/metrics", s.requireAgentAuth(s.handleMetrics))
	s.Router.HandleFunc("/api/v1/agent/command", s.requireAgentAuth(s.handleAgentCommand))
	s.Router.HandleFunc("/api/v1/agent/command/result", s.requireAgentAuth(s.handleCommandResult))
	s.Router.HandleFunc("/api/v1/agent/register", s.handleAgentRegister)
	s.Router.HandleFunc("/api/v1/admin/logs", s.handleAdminTriggerLogs)
	s.Router.HandleFunc("/api/v1/admin/disk", s.handleAdminTriggerDisk)
	s.Router.HandleFunc("/api/v1/admin/network", s.handleAdminTriggerNetwork)
	s.Router.HandleFunc("/api/v1/admin/tokens", s.handleGenerateToken)
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
