package server

import (
	"net/http"
	"slices"

	"github.com/nhdewitt/spectra/internal/database"
	"golang.org/x/crypto/bcrypt"
)

// handleListUsers returns all users (without password hashes).
//
// GET /api/v1/admin/users
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	caller, _ := userFromContext(r.Context())

	rows, err := s.DB.ListUsersWithLastLogin(r.Context())
	if err != nil {
		s.dbError(w, err, "handleListUsers")
		return
	}

	var filtered []database.ListUsersWithLastLoginRow
	if caller.Role == "viewer" {
		for _, row := range rows {
			if formatUUID(row.ID) == caller.ID {
				filtered = append(filtered, row)
				break
			}
		}
	} else {
		for _, row := range rows {
			if caller.Role == "superadmin" {
				filtered = append(filtered, row)
			} else if row.Role != "superadmin" {
				filtered = append(filtered, row)
			}
		}
	}

	respondJSON(w, http.StatusOK, filtered)
}

// handleCreateUser creates a new user account.
//
// POST /api/v1/admin/users
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	caller, ok := userFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decodeJSONBody(r, &req, maxStandardBody); err != nil {
		http.Error(w, err.Error(), badBodyStatus(err))
		return
	}

	if req.Username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		req.Role = RoleViewer
	}
	if !isValidRole(req.Role) {
		http.Error(w, "role must be superadmin, admin, or viewer", http.StatusBadRequest)
		return
	}

	if caller.Role == RoleAdmin && req.Role != RoleViewer {
		http.Error(w, "admins can only create viewer accounts", http.StatusForbidden)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.Logger.Error("failed to hash password", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := s.DB.CreateUser(r.Context(), database.CreateUserParams{
		Username: req.Username,
		Password: string(hash),
		Role:     req.Role,
	}); err != nil {
		if isPgUniqueViolation(err) {
			http.Error(w, "username already exists", http.StatusConflict)
			return
		}
		s.dbError(w, err, "handleCreateUser")
		return
	}

	s.Logger.Info("user created", "username", req.Username, "role", req.Role, "created_by", caller.Username)
	w.WriteHeader(http.StatusCreated)
}

// handleDeleteUser removes a user account.
// Superadmins can delete admins and viewers. Admins can only delete viewers.
// Nobody can delete a superadmin except another superadmin.
// Can't delete yourself. Can't delete the last superadmin.
//
// DELETE /api/v1/admin/users/{id}
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	targetID, err := parsePathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	caller, ok := userFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	if caller.ID == targetID {
		http.Error(w, "cannot delete your own account", http.StatusBadRequest)
		return
	}

	target, err := s.DB.GetUserByID(r.Context(), mustUUID(targetID))
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	roles := deletableRoles(caller.Role)
	if !slices.Contains(roles, target.Role) {
		http.Error(w, "cannot delete "+target.Role+" accounts", http.StatusForbidden)
		return
	}

	// DeleteUser enforces roles and the last-superadmin rule itself, since the target's
	// role can change after the read above. Sessions go with the user (ON DELETE CASCADE),
	// so a refused delete leaves them intact.
	rows, err := s.DB.DeleteUser(r.Context(), database.DeleteUserParams{
		ID:    mustUUID(targetID),
		Roles: roles,
	})
	if err != nil {
		s.dbError(w, err, "handleDeleteUser")
		return
	}
	if rows == 0 {
		// re-read so a role changed since the check reports as forbidden
		current, err := s.DB.GetUserByID(r.Context(), mustUUID(targetID))
		switch {
		case err != nil:
			http.Error(w, "user not found", http.StatusNotFound)
		case !slices.Contains(roles, current.Role):
			http.Error(w, "cannot delete "+target.Role+" accounts", http.StatusForbidden)
		case current.Role == RoleSuperAdmin:
			http.Error(w, "cannot delete the last superadmin", http.StatusBadRequest)
		default:
			http.Error(w, "user changed during delete, retry", http.StatusConflict)
		}
		return
	}

	s.Logger.Info("user deleted", "user_id", targetID, "target_role", target.Role, "deleted_by", caller.Username)
	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateUserRole changes a user's role.
// Superadmin only. Can't demote the last superadmin.
//
// PUT /api/v1/admin/users/{id}/role
func (s *Server) handleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	targetID, err := parsePathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	caller, ok := userFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSONBody(r, &req, maxStandardBody); err != nil {
		http.Error(w, err.Error(), badBodyStatus(err))
		return
	}
	if !isValidRole(req.Role) {
		http.Error(w, "role must be superadmin, admin, or viewer", http.StatusBadRequest)
		return
	}

	target, err := s.DB.GetUserByID(r.Context(), mustUUID(targetID))
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// UpdateUserRole refuses to demote the last superadmin itself.
	rows, err := s.DB.UpdateUserRole(r.Context(), database.UpdateUserRoleParams{
		ID:   mustUUID(targetID),
		Role: req.Role,
	})
	if err != nil {
		s.dbError(w, err, "handleUpdateUserRole")
		return
	}
	if rows == 0 {
		if target.Role == RoleSuperAdmin && req.Role != RoleSuperAdmin {
			http.Error(w, "cannot demote the last superadmin", http.StatusBadRequest)
			return
		}
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	s.Logger.Info("user role updated", "user_id", targetID, "old_role", target.Role, "new_role", req.Role, "updated_by", caller.Username)
	w.WriteHeader(http.StatusNoContent)
}
