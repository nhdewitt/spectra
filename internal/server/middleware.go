package server

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) requireAgentAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.Header.Get("X-Agent-ID")
		secret := r.Header.Get("X-Agent-Secret")

		if agentID == "" || secret == "" {
			http.Error(w, "missing agent credentials", http.StatusUnauthorized)
			return
		}

		hash, err := s.DB.GetAgentSecret(r.Context(), uuidParam(agentID))
		if err != nil {
			http.Error(w, "invalid agent credentials", http.StatusUnauthorized)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)); err != nil {
			http.Error(w, "invalid agent credentials", http.StatusUnauthorized)
			return
		}

		s.DB.TouchLastSeen(r.Context(), uuidParam(agentID))
		next(w, r)
	}
}
