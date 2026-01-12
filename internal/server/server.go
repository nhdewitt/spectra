package server

import (
	"fmt"
	"net/http"
	"time"
)

type Config struct {
	Port int
}

type Server struct {
	Config Config
	Store  *AgentStore
	Router *http.ServeMux
}

func New(cfg Config) *Server {
	s := &Server{
		Config: cfg,
		Store:  NewAgentStore(),
		Router: http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.Router.HandleFunc("/api/v1/metrics", s.handleMetrics)
	s.Router.HandleFunc("/api/v1/agent/command", s.handleAgentCommand)
	s.Router.HandleFunc("/api/v1/agent/command_result", s.handleCommandResult)
	s.Router.HandleFunc("/api/v1/agent/register", s.handleAgentRegister)
	s.Router.HandleFunc("/admin/trigger_logs", s.handleAdminTriggerLogs)
	s.Router.HandleFunc("/admin/trigger_disk", s.handleAdminTriggerDisk)
	s.Router.HandleFunc("/admin/trigger_network", s.handleAdminTriggerNetwork)
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
