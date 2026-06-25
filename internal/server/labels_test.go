package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/labels"
)

func TestHandleListAgentLabels(t *testing.T) {
	t.Run("invalid agent id", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/not-a-uuid/labels", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if mock.ListAgentLabelsCount != 0 {
			t.Errorf("DB should not be called for invalid id")
		}
	})

	t.Run("db error", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.QueryErr = errors.New("db down")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testAgentUUID+"/labels", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("success with labels", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.ListAgentLabelsReturn = []database.ListAgentLabelsRow{
			{Key: "os", Value: "linux", Source: "auto", UpdatedAt: tsNow()},
			{Key: "env", Value: "prod", Source: "user", UpdatedAt: tsNow()},
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testAgentUUID+"/labels", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		var got []labelDTO
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[0].Key != "os" || got[0].Source != "auto" {
			t.Errorf("got[0] = %+v, want os/auto", got[0])
		}
		if got[1].Key != "env" || got[1].Source != "user" {
			t.Errorf("got[1] = %+v, want env/user", got[1])
		}
	})

	t.Run("empty returns 200 with []", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+testAgentUUID+"/labels", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var got []labelDTO
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got == nil {
			t.Errorf("response should be empty array, not null")
		}
	})
}

func TestHandleListLabelKeys(t *testing.T) {
	t.Run("db error", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.QueryErr = errors.New("db down")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labels/keys", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.ListLabelKeysReturn = []database.ListLabelKeysRow{
			{Key: "os", Source: "auto"},
			{Key: "env", Source: "user"},
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labels/keys", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var got []labelKeyDTO
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got) != 2 || got[0].Key != "os" || got[1].Key != "env" {
			t.Errorf("got = %+v", got)
		}
	})
}

func TestHandleListLabelValues(t *testing.T) {
	t.Run("missing key param", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labels/values", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if mock.ListLabelValuesForKeyCount != 0 {
			t.Errorf("DB should not be called when key param missing")
		}
	})

	t.Run("db error", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.QueryErr = errors.New("db down")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labels/values?key=env", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.ListLabelValuesForKeyReturn = []string{"prod", "staging", "dev"}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labels/values?key=env", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var got []string
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got) != 3 || got[0] != "prod" {
			t.Errorf("got = %v", got)
		}
	})

	t.Run("nil normalized to []", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		// ListLabelValuesForKeyReturn is nil by default

		req := httptest.NewRequest(http.MethodGet, "/api/v1/labels/values?key=env", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		// Response body should be "[]", not "null"
		body := rec.Body.String()
		if body[0] != '[' {
			t.Errorf("body should start with '[', got %q", body)
		}
	})
}

func TestHandlePutAgentLabel(t *testing.T) {
	t.Run("invalid agent id", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)

		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/agents/not-a-uuid/labels/env", putBody("prod"))
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)

		req := httptest.NewRequest(http.MethodPut,
			"/api/v1/admin/agents/"+testAgentUUID+"/labels/env",
			bytes.NewBufferString("not json"))
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("reserved key returns 403", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)

		req := httptest.NewRequest(http.MethodPut,
			"/api/v1/admin/agents/"+testAgentUUID+"/labels/os",
			putBody("linux"))
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
		if mock.UpsertUserLabelCount != 0 {
			t.Errorf("DB should not be called for reserved key")
		}
	})

	t.Run("invalid key format returns 400", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)

		req := httptest.NewRequest(http.MethodPut,
			"/api/v1/admin/agents/"+testAgentUUID+"/labels/Env", // uppercase
			putBody("prod"))
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("empty value returns 400", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)

		req := httptest.NewRequest(http.MethodPut,
			"/api/v1/admin/agents/"+testAgentUUID+"/labels/env",
			putBody(""))
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("conflict with auto label returns 409", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.UpsertUserLabelErr = pgx.ErrNoRows

		req := httptest.NewRequest(http.MethodPut,
			"/api/v1/admin/agents/"+testAgentUUID+"/labels/env",
			putBody("prod"))
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("status = %d, want 409", rec.Code)
		}
	})

	t.Run("db error returns 500", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.UpsertUserLabelErr = errors.New("db down")

		req := httptest.NewRequest(http.MethodPut,
			"/api/v1/admin/agents/"+testAgentUUID+"/labels/env",
			putBody("prod"))
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.UpsertUserLabelReturn = database.AgentLabel{
			Key:       "env",
			Value:     "prod",
			Source:    "user",
			UpdatedAt: tsNow(),
		}

		req := httptest.NewRequest(http.MethodPut,
			"/api/v1/admin/agents/"+testAgentUUID+"/labels/env",
			putBody("prod"))
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		var got labelDTO
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Key != "env" || got.Value != "prod" || got.Source != "user" {
			t.Errorf("got = %+v", got)
		}
		// Verify the handler passed the right params to the DB
		if mock.LastUpsertUserLabelParams.Key != "env" {
			t.Errorf("DB called with key=%q, want env", mock.LastUpsertUserLabelParams.Key)
		}
		if mock.LastUpsertUserLabelParams.Value != "prod" {
			t.Errorf("DB called with value=%q, want prod", mock.LastUpsertUserLabelParams.Value)
		}
	})
}

func TestHandleDeleteAgentLabel(t *testing.T) {
	t.Run("invalid agent id", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)

		req := httptest.NewRequest(http.MethodDelete,
			"/api/v1/admin/agents/not-a-uuid/labels/env", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.DeleteUserLabelErr = errors.New("db down")

		req := httptest.NewRequest(http.MethodDelete,
			"/api/v1/admin/agents/"+testAgentUUID+"/labels/env", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.DeleteUserLabelRows = 0
		mock.GetAgentLabelErr = pgx.ErrNoRows

		req := httptest.NewRequest(http.MethodDelete,
			"/api/v1/admin/agents/"+testAgentUUID+"/labels/env", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("auto label returns 403", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.DeleteUserLabelRows = 0
		mock.GetAgentLabelReturn = database.GetAgentLabelRow{
			Key:    "os",
			Value:  "linux",
			Source: "auto",
		}

		req := httptest.NewRequest(http.MethodDelete,
			"/api/v1/admin/agents/"+testAgentUUID+"/labels/os", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})

	t.Run("success returns 204", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.DeleteUserLabelRows = 1

		req := httptest.NewRequest(http.MethodDelete,
			"/api/v1/admin/agents/"+testAgentUUID+"/labels/env", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rec.Code)
		}
		// GetAgentLabel should NOT be called on the success path
		if mock.GetAgentLabelCount != 0 {
			t.Errorf("GetAgentLabel should not be called on successful delete")
		}
	})
}

func TestSyncAutoLabelsOnRegister(t *testing.T) {
	t.Run("success calls DB and updates cache", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		info := labels.AgentInfo{
			OS:           "linux",
			Arch:         "amd64",
			Hardware:     "raspberry-pi",
			AgentVersion: "1.0.0",
		}

		err := s.syncAutoLabelsOnRegister(context.Background(), testAgentUUID, info)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mock.ReplaceAutoLabelsCount != 1 {
			t.Errorf("ReplaceAutoLabels called %d times, want 1", mock.ReplaceAutoLabelsCount)
		}
		if len(mock.LastReplaceAutoLabelsParams.Keys) != 4 {
			t.Errorf("Keys len = %d, want 4", len(mock.LastReplaceAutoLabelsParams.Keys))
		}
		// Cache should reflect the version
		if s.versionCache.Changed(testAgentUUID, "1.0.0") {
			t.Error("cache should not report drift after successful sync")
		}
	})

	t.Run("db error leaves cache untouched", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		mock.Err = errors.New("db down")
		info := labels.AgentInfo{OS: "linux", AgentVersion: "1.0.0"}

		err := s.syncAutoLabelsOnRegister(context.Background(), testAgentUUID, info)
		if err == nil {
			t.Fatal("expected error")
		}
		// Cache should NOT be updated → next call still sees drift
		if !s.versionCache.Changed(testAgentUUID, "1.0.0") {
			t.Error("cache should not be updated on DB error")
		}
	})

	t.Run("empty version skips cache update", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		info := labels.AgentInfo{OS: "linux"} // no version

		err := s.syncAutoLabelsOnRegister(context.Background(), testAgentUUID, info)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mock.ReplaceAutoLabelsCount != 1 {
			t.Errorf("ReplaceAutoLabels should still be called even with empty version")
		}
		if s.versionCache.Len() != 0 {
			t.Errorf("cache len = %d, want 0 (no version means no cache entry)",
				s.versionCache.Len())
		}
	})
}

func TestSyncAgentVersionLabel(t *testing.T) {
	t.Run("empty version is no-op", func(t *testing.T) {
		s, _, _, mock := newTestServer()

		err := s.syncAgentVersionLabel(context.Background(), testAgentUUID, "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if mock.UpsertAutoLabelCount != 0 {
			t.Errorf("UpsertAutoLabel should not be called for empty version")
		}
	})

	t.Run("first sighting triggers upsert", func(t *testing.T) {
		s, _, _, mock := newTestServer()

		err := s.syncAgentVersionLabel(context.Background(), testAgentUUID, "1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mock.UpsertAutoLabelCount != 1 {
			t.Errorf("UpsertAutoLabel called %d times, want 1", mock.UpsertAutoLabelCount)
		}
	})

	t.Run("cached version is no-op", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		s.versionCache.Update(testAgentUUID, "1.0.0")

		err := s.syncAgentVersionLabel(context.Background(), testAgentUUID, "1.0.0")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if mock.UpsertAutoLabelCount != 0 {
			t.Errorf("UpsertAutoLabel should not be called when version unchanged")
		}
	})

	t.Run("drift triggers upsert", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		s.versionCache.Update(testAgentUUID, "1.0.0")

		err := s.syncAgentVersionLabel(context.Background(), testAgentUUID, "1.0.1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mock.UpsertAutoLabelCount != 1 {
			t.Errorf("UpsertAutoLabel called %d times, want 1", mock.UpsertAutoLabelCount)
		}
		if mock.LastUpsertAutoLabelParams.Value != "1.0.1" {
			t.Errorf("upsert value = %q, want 1.0.1", mock.LastUpsertAutoLabelParams.Value)
		}
	})

	t.Run("db error leaves cache untouched for retry", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		mock.Err = errors.New("db down")

		err := s.syncAgentVersionLabel(context.Background(), testAgentUUID, "1.0.0")
		if err == nil {
			t.Fatal("expected error")
		}
		// Cache should NOT have been updated → next call still sees drift
		if !s.versionCache.Changed(testAgentUUID, "1.0.0") {
			t.Error("cache should not be updated on DB error")
		}
	})
}

func TestForgetAgentLabels(t *testing.T) {
	s, _, _, _ := newTestServer()
	s.versionCache.Update(testAgentUUID, "1.0.0")

	s.forgetAgentLabels(testAgentUUID)

	if !s.versionCache.Changed(testAgentUUID, "1.0.0") {
		t.Error("after forget, version should look new")
	}
}

func TestHandleListAllAgentLabels(t *testing.T) {
	t.Run("db error", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		mock.QueryErr = errors.New("db down")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/labels", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("success groups by agent id", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)

		a1 := newTestUUID()
		a2 := newTestUUID()
		id1 := formatUUID(a1)
		id2 := formatUUID(a2)

		// Two agents, interleaved; auto first within each (as the query orders).
		mock.ListAllAgentLabelsReturn = []database.ListAllAgentLabelsRow{
			{AgentID: a1, Key: "os", Value: "linux", Source: "auto"},
			{AgentID: a1, Key: "env", Value: "prod", Source: "user"},
			{AgentID: a2, Key: "os", Value: "windows", Source: "auto"},
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/labels", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}

		var got map[string][]bulkLabelDTO
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d agents, want 2", len(got))
		}
		if len(got[id1]) != 2 {
			t.Fatalf("agent1 labels = %d, want 2", len(got[id1]))
		}
		// Order within an agent is preserved from the query.
		if got[id1][0].Key != "os" || got[id1][0].Source != "auto" {
			t.Errorf("agent1[0] = %+v, want os/auto", got[id1][0])
		}
		if got[id1][1].Key != "env" || got[id1][1].Source != "user" {
			t.Errorf("agent1[1] = %+v, want env/user", got[id1][1])
		}
		if len(got[id2]) != 1 || got[id2][0].Value != "windows" {
			t.Errorf("agent2 = %+v, want one windows label", got[id2])
		}
	})

	t.Run("empty returns 200 with object", func(t *testing.T) {
		s, _, _, mock := newTestServer()
		setupTestSession(mock)
		// ListAllAgentLabelsReturn is nil by default

		req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/labels", nil)
		authedRequest(req)
		rec := httptest.NewRecorder()
		s.Router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		// Body should be "{}", not "null" (the handler inits a non-nil map).
		body := rec.Body.String()
		if len(body) == 0 || body[0] != '{' {
			t.Errorf("body should start with '{', got %q", body)
		}
		var got map[string][]bulkLabelDTO
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got == nil {
			t.Errorf("response should be an empty object, not null")
		}
	})
}
