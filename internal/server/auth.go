package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"net/netip"
	"strings"
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
)

// Context-based user identity

type userContextKeyType struct{}

var userContextKey userContextKeyType

var dummyHash []byte

type userContext struct {
	ID       string
	Username string
	Role     string
}

func init() {
	var err error
	dummyHash, err = bcrypt.GenerateFromPassword([]byte("dummypass"), bcrypt.DefaultCost)
	if err != nil {
		panic("failed to generate dummy bcrypt hash")
	}
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
		if session.IpAddress != s.clientIP(r) {
			if err := s.DB.DeleteSession(r.Context(), cookie.Value); err != nil {
				s.Logger.Error("failed to delete session", "error", err)
			}
			s.clearSessionCookie(w)
			s.Logger.Warn("session invalidated: IP mismatch",
				"username", session.Username,
				"session_ip", session.IpAddress,
				"request_ip", s.clientIP(r),
			)
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
	ip := s.clientIP(r)

	if err := s.LoginTracker.check(ip); err != nil {
		s.Logger.Warn("login locked out", "ip", ip)
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSONBody(r, &req, maxAuthBody); err != nil {
		http.Error(w, "invalid request", badBodyStatus(err))
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}

	user, err := s.DB.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		// Constant-time comparison to prevent timing attacks
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))
		s.LoginTracker.recordFailure(ip)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		s.LoginTracker.recordFailure(ip)
		s.Logger.Warn("login failed", "username", req.Username, "ip", ip)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	s.LoginTracker.recordSuccess(ip)
	s.Logger.Info("login successful", "username", user.Username, "ip", ip)

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
		Secure:   s.secureCookies,
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

	if err := s.DB.DeleteSession(r.Context(), cookie.Value); err != nil {
		s.Logger.Error("failed to delete session on logout", "error", err)
	}
	s.clearSessionCookie(w)
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
		"id":       u.ID,
		"username": u.Username,
		"role":     u.Role,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

// useSecureCookies decides whether the session cookie carries the Secure
// attribute, which keeps the browser from ever sending it over plain HTTP.
//
// It is derived rather than configured because both real deployments answer it
// correctly on their own. Serving TLS directly means HTTPS, and an https
// ExternalURL means a reverse proxy is terminating TLS in front of a server
// that may itself be listening on plain HTTP. Hardcoding true would lock out
// any plain-HTTP deployment, since the browser would hold the cookie back on
// every request and the login would appear to succeed and then immediately fail.
func useSecureCookies(cfg Config) bool {
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(cfg.ExternalURL)), "https://")
}

// clientIP returns the address a request should be attributed to for rate
// limiting, login lockout, and session IP binding.
//
// X-Forwarded-For and X-Real-IP are only consulted when the immediate peer is
// itself a configured trusted proxy. Any other caller can set those headers to
// a different value on every request, which hands out a fresh lockout and
// rate-limit bucket each time and makes session IP binding meaningless.
func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	peer, err := netip.ParseAddr(host)
	if err != nil || !s.isTrustedProxy(peer) {
		if r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-IP") != "" {
			s.xffWarnOnce.Do(func() {
				s.Logger.Warn("ignoring forwarded-for headers from an untrusted peer; set trusted_proxies if this server sits behind a reverse proxy",
					"peer", host)
			})
		}
		return host
	}

	// Walk right to left. The rightmost entry was observed by the nearest
	// proxy and is the last hop an external client cannot forge; everything
	// left of it is only as trustworthy as the proxy that appended it. Stop at
	// the first hop that isn't a proxy we trust.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		hops := strings.Split(xff, ",")
		for i := len(hops) - 1; i >= 0; i-- {
			addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
			if err != nil {
				continue
			}
			if !s.isTrustedProxy(addr) {
				return addr.String()
			}
		}
	}

	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		if addr, err := netip.ParseAddr(xri); err == nil {
			return addr.String()
		}
	}

	// Every hop was a trusted proxy, or no usable header was present.
	return host
}

func (s *Server) isTrustedProxy(addr netip.Addr) bool {
	if len(s.trustedProxies) == 0 {
		return false
	}
	addr = addr.Unmap()
	for _, p := range s.trustedProxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// parseTrustedProxies converts configured CIDRs and bare addresses into
// prefixes, returning any entries it could not parse so the caller can log
// them. Bad entries are dropped rather than fatal: dropping one only ever
// narrows what the server trusts.
func parseTrustedProxies(entries []string) (prefixes []netip.Prefix, invalid []string) {
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if p, err := netip.ParsePrefix(e); err == nil {
			prefixes = append(prefixes, p.Masked())
			continue
		}
		if addr, err := netip.ParseAddr(e); err == nil {
			addr = addr.Unmap()
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		invalid = append(invalid, e)
	}
	return
}

func (s *Server) tokenOrAuth(next http.HandlerFunc) http.HandlerFunc {
	authed := s.requireUserAuth(next)
	return func(w http.ResponseWriter, r *http.Request) {
		// Registration token
		if token := r.URL.Query().Get("token"); token != "" {
			if s.Tokens.Peek(token) {
				next(w, r)
				return
			}
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}
		// Agent auth
		if r.Header.Get("X-Agent-ID") != "" {
			s.requireAgentAuth(next)(w, r)
			return
		}
		// User session
		authed(w, r)
	}
}
