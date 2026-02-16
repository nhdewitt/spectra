package server

import "net/http"

func (s *Server) requireAgentAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.Header.Get("X-Agent-ID")
		secret := r.Header.Get("X-Agent-Secret")

		if agentID == "" || secret == "" {
			http.Error(w, "missing agent credentials", http.StatusUnauthorized)
			return
		}
		if !s.Store.Authenticate(agentID, secret) {
			http.Error(w, "invalid agent credentials", http.StatusUnauthorized)
			return
		}

		s.Store.TouchLastSeen(agentID)

		next(w, r)
	}
}
