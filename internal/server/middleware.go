package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
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

		authOK := false

		// Try SHA-256 first
		shaHash, err := s.DB.GetAgentSecretSHA256(r.Context(), id)
		if err == nil && len(shaHash) == sha256.Size {
			presented := sha256.Sum256([]byte(secret))
			authOK = subtle.ConstantTimeCompare(presented[:], shaHash) == 1
		} else {
			// Fall back (old agents)
			bcryptHash, err := s.DB.GetAgentSecret(r.Context(), id)
			if err == nil && bcrypt.CompareHashAndPassword([]byte(bcryptHash), []byte(secret)) == nil {
				authOK = true
				// upgrade to SHA-256
				sum := sha256.Sum256([]byte(secret))
				if err := s.DB.SetAgentSecretSHA256(r.Context(), database.SetAgentSecretSHA256Params{
					ID:           id,
					SecretSha256: sum[:],
				}); err != nil {
					log.Printf("Failed upgrading agent %s to SHA-256: %v", agentID, err)
				}
			}
		}

		if !authOK {
			http.Error(w, "invalid agent credentials", http.StatusUnauthorized)
			return
		}

		if err := s.DB.TouchLastSeenIfStale(r.Context(), id); err != nil {
			log.Printf("Failed to update agent %s last_seen: %v", agentID, err)
		}
		next(w, r)
	}
}
