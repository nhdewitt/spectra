package server

import (
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
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

		var id pgtype.UUID
		if err := id.Scan(agentID); err != nil {
			http.Error(w, "invalid agent ID", http.StatusUnauthorized)
			return
		}

		hash, err := s.DB.GetAgentSecret(r.Context(), id)
		if err != nil {
			http.Error(w, "invalid agent credentials", http.StatusUnauthorized)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)); err != nil {
			http.Error(w, "invalid agent credentials", http.StatusUnauthorized)
			return
		}

		if err := s.DB.TouchLastSeen(r.Context(), id); err != nil {
			log.Printf("Failed to update agent %s last_seen: %v", agentID, err)
		}
		next(w, r)
	}
}
