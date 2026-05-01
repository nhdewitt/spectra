//go:build windows

package main

import (
	"log"

	"github.com/nhdewitt/spectra/internal/agent"
	"golang.org/x/sys/windows/svc"
)

type spectraService struct {
	agent *agent.Agent
}

func (s *spectraService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	go func() {
		if err := s.agent.Start(); err != nil {
			log.Printf("Agent exited with error: %v", err)
		}
	}()

	changes <- svc.Status{
		State:   svc.Running,
		Accepts: svc.AcceptStop | svc.AcceptShutdown,
	}

	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			s.agent.Shutdown()
			return false, 0
		}
	}

	return false, 0
}

func runService(a *agent.Agent) error {
	return svc.Run("SpectraAgent", &spectraService{agent: a})
}

func isWindowsService() bool {
	is, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return is
}
