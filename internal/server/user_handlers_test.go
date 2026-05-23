package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

var (
	superadminUID = pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	adminUID      = pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	viewerUID     = pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	superadminID  = formatUUID(superadminUID)
	adminID       = formatUUID(adminUID)
	viewerID      = formatUUID(viewerUID)
)

func setupSuperadminSession(mock *MockDB) {
	setupTestSessionWithRole(mock, "super-token", "superadmin", RoleSuperAdmin, testSessionIP, superadminUID)
}

func setupAdminSession(mock *MockDB) {
	setupTestSessionWithRole(mock, "admin-token", "admin", RoleAdmin, testSessionIP, adminUID)
}

func setupViewerSession(mock *MockDB) {
	setupTestSessionWithRole(mock, "viewer-token", "viewer", RoleViewer, testSessionIP, viewerUID)
}

func superadminRequest(req *http.Request) *http.Request {
	return authedRequestAs(req, "super-token", testSessionIP)
}

func adminRequest(req *http.Request) *http.Request {
	return authedRequestAs(req, "admin-token", testSessionIP)
}

func viewerRequest(req *http.Request) *http.Request {
	return authedRequestAs(req, "viewer-token", testSessionIP)
}

func seedUser(mock *MockDB, id pgtype.UUID, username, role string) {
	if mock.UserByID == nil {
		mock.UserByID = make(map[pgtype.UUID]database.GetUserByIDRow)
	}
	mock.UserByID[id] = database.GetUserByIDRow{
		ID:       id,
		Username: username,
		Role:     role,
	}
}

func seedUserRows(mock *MockDB) {
	mock.UserRowsWithLastLogin = []database.ListUsersWithLastLoginRow{
		{ID: superadminUID, Username: "superadmin", Role: RoleSuperAdmin},
		{ID: adminUID, Username: "admin", Role: RoleAdmin},
		{ID: viewerUID, Username: "viewer", Role: RoleViewer},
	}
}

func postJSON(path string, body any) *http.Request {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func putJSON(path string, body any) *http.Request {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestHandleListUsers_SuperadminSeesAll(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)
	seedUserRows(mock)

	req := superadminRequest(httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}

	var users []database.ListUsersWithLastLoginRow
	if err := json.NewDecoder(rec.Body).Decode(&users); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(users) != 3 {
		t.Errorf("superadmin should see 3 users, got %d", len(users))
	}
}

func TestHandleListUsers_AdminSeesNoSuperadmins(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupAdminSession(mock)
	seedUserRows(mock)

	req := adminRequest(httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}

	var users []database.ListUsersWithLastLoginRow
	if err := json.NewDecoder(rec.Body).Decode(&users); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("admin should see 2 users, got %d", len(users))
	}
	for _, u := range users {
		if u.Role == RoleSuperAdmin {
			t.Errorf("admin should not see superadmin user %s", u.Username)
		}
	}
}

func TestHandleListUsers_ViewerSeesOnlySelf(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupViewerSession(mock)
	seedUserRows(mock)

	req := viewerRequest(httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}

	var users []database.ListUsersWithLastLoginRow
	if err := json.NewDecoder(rec.Body).Decode(&users); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("viewer should see 1 user, got %d", len(users))
	}
	if formatUUID(users[0].ID) != viewerID {
		t.Errorf("viewer should only see themselves, got %s", users[0].Username)
	}
}

func TestHandleListUsers_ViewerSeesNothingIfNotInList(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupViewerSession(mock)
	// Seed only superadmin and admin — viewer not in the list
	mock.UserRowsWithLastLogin = []database.ListUsersWithLastLoginRow{
		{ID: superadminUID, Username: "superadmin", Role: RoleSuperAdmin},
		{ID: adminUID, Username: "admin", Role: RoleAdmin},
	}

	req := viewerRequest(httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}

	var users []database.ListUsersWithLastLoginRow
	if err := json.NewDecoder(rec.Body).Decode(&users); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("viewer not in list should see 0 users, got %d", len(users))
	}
}

func TestHandleListUsers_DBError(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)
	mock.QueryErr = errors.New("db down")

	req := superadminRequest(httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

func TestHandleCreateUser_SuperadminCreatesAdmin(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)

	req := superadminRequest(postJSON("/api/v1/admin/users", map[string]string{
		"username": "newadmin",
		"password": "password1234",
		"role":     "admin",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateUser_SuperadminCreatesSuperadmin(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)

	req := superadminRequest(postJSON("/api/v1/admin/users", map[string]string{
		"username": "newsuper",
		"password": "password1234",
		"role":     "superadmin",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateUser_AdminCreatesViewer(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupAdminSession(mock)

	req := adminRequest(postJSON("/api/v1/admin/users", map[string]string{
		"username": "newviewer",
		"password": "password1234",
		"role":     "viewer",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateUser_AdminCannotCreateAdmin(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupAdminSession(mock)

	req := adminRequest(postJSON("/api/v1/admin/users", map[string]string{
		"username": "sneaky",
		"password": "password1234",
		"role":     "admin",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestHandleCreateUser_ViewerForbidden(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupViewerSession(mock)

	req := viewerRequest(postJSON("/api/v1/admin/users", map[string]string{
		"username": "nope",
		"password": "password1234",
		"role":     "viewer",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestHandleCreateUser_MissingUsername(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)

	req := superadminRequest(postJSON("/api/v1/admin/users", map[string]string{
		"password": "password1234",
		"role":     "viewer",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleCreateUser_ShortPassword(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)

	req := superadminRequest(postJSON("/api/v1/admin/users", map[string]string{
		"username": "newuser",
		"password": "short",
		"role":     "viewer",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleCreateUser_InvalidRole(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)

	req := superadminRequest(postJSON("/api/v1/admin/users", map[string]string{
		"username": "newuser",
		"password": "password1234",
		"role":     "root",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleCreateUser_DefaultsToViewer(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)

	req := superadminRequest(postJSON("/api/v1/admin/users", map[string]string{
		"username": "newuser",
		"password": "password1234",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteUser_SuperadminDeletesViewer(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)
	seedUser(mock, viewerUID, "viewer", RoleViewer)

	req := superadminRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+viewerID, nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if mock.DeleteUserCount != 1 {
		t.Errorf("DeleteUser called %d times, want 1", mock.DeleteUserCount)
	}
}

func TestHandleDeleteUser_SuperadminDeletesAdmin(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)
	seedUser(mock, adminUID, "admin", RoleAdmin)

	req := superadminRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+adminID, nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteUser_AdminDeletesViewer(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupAdminSession(mock)
	seedUser(mock, viewerUID, "viewer", RoleViewer)

	req := adminRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+viewerID, nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteUser_AdminCannotDeleteAdmin(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupAdminSession(mock)

	otherAdmin := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	seedUser(mock, otherAdmin, "otheradmin", RoleAdmin)

	req := adminRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+formatUUID(otherAdmin), nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestHandleDeleteUser_AdminCannotDeleteSuperadmin(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupAdminSession(mock)
	seedUser(mock, superadminUID, "superadmin", RoleSuperAdmin)

	req := adminRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+superadminID, nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestHandleDeleteUser_CannotDeleteSelf(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)
	seedUser(mock, superadminUID, "superadmin", RoleSuperAdmin)

	req := superadminRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+superadminID, nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleDeleteUser_CannotDeleteLastSuperadmin(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)
	mock.SuperAdmins = 1

	otherSuper := pgtype.UUID{Bytes: [16]byte{8}, Valid: true}
	seedUser(mock, otherSuper, "othersuper", RoleSuperAdmin)

	// Need a second superadmin session so caller != target
	req := superadminRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+formatUUID(otherSuper), nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteUser_SuperadminDeletesOtherSuperadmin(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)
	mock.SuperAdmins = 2

	otherSuper := pgtype.UUID{Bytes: [16]byte{8}, Valid: true}
	seedUser(mock, otherSuper, "othersuper", RoleSuperAdmin)

	req := superadminRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+formatUUID(otherSuper), nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteUser_ViewerForbidden(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupViewerSession(mock)
	seedUser(mock, adminUID, "admin", RoleAdmin)

	req := viewerRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+adminID, nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestHandleDeleteUser_NotFound(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)
	// Don't seed any user — GetUserByID will return not found

	req := superadminRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+viewerID, nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestHandleUpdateUserRole_SuperadminPromotesToAdmin(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)
	seedUser(mock, viewerUID, "viewer", RoleViewer)

	req := superadminRequest(putJSON("/api/v1/admin/users/"+viewerID+"/role", map[string]string{
		"role": "admin",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if mock.UpdateUserRoleCount != 1 {
		t.Errorf("UpdateUserRole called %d times, want 1", mock.UpdateUserRoleCount)
	}
}

func TestHandleUpdateUserRole_AdminForbidden(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupAdminSession(mock)
	seedUser(mock, viewerUID, "viewer", RoleViewer)

	req := adminRequest(putJSON("/api/v1/admin/users/"+viewerID+"/role", map[string]string{
		"role": "admin",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want 403", rec.Code)
	}
}

func TestHandleUpdateUserRole_CannotDemoteLastSuperadmin(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)
	mock.SuperAdmins = 1

	otherSuper := pgtype.UUID{Bytes: [16]byte{8}, Valid: true}
	seedUser(mock, otherSuper, "othersuper", RoleSuperAdmin)

	req := superadminRequest(putJSON("/api/v1/admin/users/"+formatUUID(otherSuper)+"/role", map[string]string{
		"role": "admin",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateUserRole_DemoteSuperadminWithMultiple(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)
	mock.SuperAdmins = 2

	otherSuper := pgtype.UUID{Bytes: [16]byte{8}, Valid: true}
	seedUser(mock, otherSuper, "othersuper", RoleSuperAdmin)

	req := superadminRequest(putJSON("/api/v1/admin/users/"+formatUUID(otherSuper)+"/role", map[string]string{
		"role": "admin",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateUserRole_InvalidRole(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)
	seedUser(mock, viewerUID, "viewer", RoleViewer)

	req := superadminRequest(putJSON("/api/v1/admin/users/"+viewerID+"/role", map[string]string{
		"role": "root",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleUpdateUserRole_NotFound(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupSuperadminSession(mock)

	req := superadminRequest(putJSON("/api/v1/admin/users/"+viewerID+"/role", map[string]string{
		"role": "admin",
	}))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}
