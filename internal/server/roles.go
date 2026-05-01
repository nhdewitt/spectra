package server

import "net/http"

const (
	RoleSuperAdmin = "superadmin"
	RoleAdmin      = "admin"
	RoleViewer     = "viewer"
)

var roleLevel = map[string]int{
	RoleViewer:     0,
	RoleAdmin:      1,
	RoleSuperAdmin: 2,
}

func isValidRole(role string) bool {
	_, ok := roleLevel[role]
	return ok
}

func hasMinRole(userRole, minRole string) bool {
	return roleLevel[userRole] >= roleLevel[minRole]
}

// requireRole checks that the authenticated user has at least the given role.
// Must be used after requireUserAuth.
func requireRole(minRole string) func(http.HandlerFunc) http.HandlerFunc {
	minLevel := roleLevel[minRole]

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			u, ok := userFromContext(r.Context())
			if !ok {
				http.Error(w, "not authenticated", http.StatusUnauthorized)
				return
			}
			if roleLevel[u.Role] < minLevel {
				http.Error(w, "insufficient permissions", http.StatusForbidden)
				return
			}
			next(w, r)
		}
	}
}
