package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName = "spectra_session"
	sessionDuration   = 24 * time.Hour
	sessionTokenBytes = 32

	maxLoginAttempts = 5
	lockoutDuration  = 15 * time.Minute

	requestsPerSecond = 10.0 // rate limit
	burst             = 30   // rate limit
)

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummypass"), bcrypt.DefaultCost)

// Context-based user identity

type userContextKeyType struct{}

var userContextKey userContextKeyType

type userContext struct {
	ID       string
	Username string
	Role     string
}

// userFromContext retrieves the authenticated user from the request context.
// Returns false if no user is set.
func userFromContext(ctx context.Context) (*userContext, bool) {
	u, ok := ctx.Value(userContextKey).(*userContext)
	return u, ok
}

// requireUserAuth validates the session cookie, checks IP binding,
// and injects user identity into the request context.
func (s *Server) requireUserAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}

		session, err := s.DB.GetSession(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, "invalid or expired session", http.StatusUnauthorized)
			return
		}

		// Verify IP
		if session.IpAddress != clientIP(r) {
			s.DB.DeleteSession(r.Context(), cookie.Value)
			clearSessionCookie(w)
			http.Error(w, "session invalidated", http.StatusUnauthorized)
			return
		}

		u := &userContext{
			ID:       formatUUID(session.UserID),
			Username: session.Username,
			Role:     session.Role,
		}
		ctx := context.WithValue(r.Context(), userContextKey, u)

		next(w, r.WithContext(ctx))
	}
}

// handleLogin authenticates a user and creates an IP-bound session.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)

	if err := s.LoginTracker.check(ip); err != nil {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}

	user, err := s.DB.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		// Constant-time comparison to prevent timing attacks
		bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))
		s.LoginTracker.recordFailure(ip)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		s.LoginTracker.recordFailure(ip)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	s.LoginTracker.recordSuccess(ip)

	// Session token
	tokenBytes := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(sessionDuration)
	if err := s.DB.CreateSession(r.Context(), database.CreateSessionParams{
		Token:  token,
		UserID: user.ID,
		ExpiresAt: pgtype.Timestamptz{
			Time:  expiresAt,
			Valid: true,
		},
		IpAddress: ip,
	}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	respondJSON(w, http.StatusOK, map[string]string{
		"username": user.Username,
		"role":     user.Role,
	})
}

// handleLogout destroys the current session.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	s.DB.DeleteSession(r.Context(), cookie.Value)
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// handleMe returns the current user's info.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := userFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"username": u.Username,
		"role":     u.Role,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
