package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

// jsonBody marshals v and returns it as a request body buffer.
func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return bytes.NewBuffer(b)
}

// seedChannel inserts a channel directly into the mock and returns its ID string.
func seedChannel(mock *MockDB, name, ctype string) string {
	ch, _ := mock.CreateAlertChannel(context.Background(), database.CreateAlertChannelParams{
		Name:   name,
		Type:   ctype,
		Config: []byte(`{"url":"https://example.com/hook"}`),
	})
	return formatUUID(ch.ID)
}

func TestHandleListAlertChannels_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/alerts/channels", nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleListAlertChannels_Unauthenticated(t *testing.T) {
	s, _, _, _ := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/channels", nil)
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestHandleCreateAlertChannel_Webhook(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := jsonBody(t, channelRequest{
		Name:   "ops-hook",
		Type:   "webhook",
		Config: json.RawMessage(`{"url":"https://example.com/hook"}`),
	})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/alerts/channels", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want 201", rec.Code)
	}
	var resp struct {
		ID     string          `json:"id"`
		Name   string          `json:"name"`
		Type   string          `json:"type"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Name != "ops-hook" || resp.Type != "webhook" {
		t.Errorf("unexpected channel: %+v", resp)
	}
}

func TestHandleCreateAlertChannel_Email(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := jsonBody(t, channelRequest{
		Name:   "ops-email",
		Type:   "email",
		Config: json.RawMessage(`{"to":"ops@example.com"}`),
	})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/alerts/channels", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201", rec.Code)
	}
}

func TestHandleCreateAlertChannel_MissingName(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := jsonBody(t, channelRequest{
		Type:   "webhook",
		Config: json.RawMessage(`{"url":"https://example.com/hook"}`),
	})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/alerts/channels", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleCreateAlertChannel_InvalidType(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := jsonBody(t, channelRequest{
		Name:   "bad",
		Type:   "carrier-pigeon",
		Config: json.RawMessage(`{}`),
	})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/alerts/channels", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleCreateAlertChannel_WebhookMissingURL(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := jsonBody(t, channelRequest{
		Name:   "no-url",
		Type:   "webhook",
		Config: json.RawMessage(`{}`),
	})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/alerts/channels", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleDeleteAlertChannel_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	id := seedChannel(mock, "doomed", "webhook")

	req := authedRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/alerts/channels/"+id, nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", rec.Code)
	}
}

func TestHandleDeleteAlertChannel_InvalidID(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/alerts/channels/not-a-uuid", nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleCreateAlertRule_GlobalOffline(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := jsonBody(t, ruleRequest{
		Name:            "fleet offline",
		Enabled:         true,
		Scope:           "global",
		ConditionType:   "agent_offline",
		ConditionParams: json.RawMessage(`{"timeout_seconds":300}`),
		CooldownSeconds: 3600,
	})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/alerts/rules", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want 201", rec.Code)
	}
}

func TestHandleCreateAlertRule_GlobalServiceDownRejected(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := jsonBody(t, ruleRequest{
		Name:            "bad global service",
		Enabled:         true,
		Scope:           "global",
		ConditionType:   "service_down",
		ConditionParams: json.RawMessage(`{"service_name":"nginx"}`),
	})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/alerts/rules", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400 (global service_down must be rejected)", rec.Code)
	}
}

func TestHandleCreateAlertRule_AgentScopedNoAgentID(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := jsonBody(t, ruleRequest{
		Name:            "missing agent",
		Scope:           "agent",
		ConditionType:   "agent_offline",
		ConditionParams: json.RawMessage(`{"timeout_seconds":300}`),
	})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/alerts/rules", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleCreateAlertRule_ServiceDownWarnsWhenNotReported(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	// Agent reports only sshd; rule targets nginx → expect a warning, not rejection.
	mock.ServicesByAgent[testAgentUUID] = []database.CurrentService{
		{AgentID: mustUUID(testAgentUUID), Name: "sshd", Status: pgtype.Text{String: "active", Valid: true}},
	}

	body := jsonBody(t, ruleRequest{
		Name:            "nginx watch",
		Enabled:         true,
		Scope:           "agent",
		AgentID:         testAgentUUID,
		ConditionType:   "service_down",
		ConditionParams: json.RawMessage(`{"service_name":"nginx"}`),
	})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/alerts/rules", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want 201", rec.Code)
	}
	var resp ruleResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Warnings) == 0 {
		t.Error("expected a warning for unreported service, got none")
	}
}

func TestHandleCreateAlertRule_ServiceDownNoWarnWhenReported(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	// Agent reports nginx → no warning expected.
	mock.ServicesByAgent[testAgentUUID] = []database.CurrentService{
		{AgentID: mustUUID(testAgentUUID), Name: "nginx", Status: pgtype.Text{String: "active", Valid: true}},
	}

	body := jsonBody(t, ruleRequest{
		Name:            "nginx watch",
		Enabled:         true,
		Scope:           "agent",
		AgentID:         testAgentUUID,
		ConditionType:   "service_down",
		ConditionParams: json.RawMessage(`{"service_name":"nginx"}`),
	})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/alerts/rules", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want 201", rec.Code)
	}
	var resp ruleResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Warnings) != 0 {
		t.Errorf("expected no warning for reported service, got: %v", resp.Warnings)
	}
}

func TestHandleCreateAlertRule_InvalidConditionType(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := jsonBody(t, ruleRequest{
		Name:            "bogus",
		Scope:           "global",
		ConditionType:   "cpu_on_fire",
		ConditionParams: json.RawMessage(`{}`),
	})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/alerts/rules", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleCreateAlertRule_NegativeCooldown(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	body := jsonBody(t, ruleRequest{
		Name:            "neg cooldown",
		Scope:           "global",
		ConditionType:   "agent_offline",
		ConditionParams: json.RawMessage(`{"timeout_seconds":300}`),
		CooldownSeconds: -5,
	})
	req := authedRequest(httptest.NewRequest(http.MethodPost, "/api/v1/alerts/rules", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

func TestHandleListAlertRules_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/alerts/rules", nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestHandleGetAlertRule_NotFound(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/alerts/rules/"+testAgentUUID, nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
}

func TestHandleSetAlertRuleEnabled_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	rule, _ := mock.CreateAlertRule(context.Background(), database.CreateAlertRuleParams{
		Name:            "toggle me",
		Enabled:         true,
		Scope:           "global",
		ConditionType:   "agent_offline",
		ConditionParams: []byte(`{"timeout_seconds":300}`),
		CooldownSeconds: 3600,
	})

	body := jsonBody(t, struct {
		Enabled bool `json:"enabled"`
	}{Enabled: false})
	req := authedRequest(httptest.NewRequest(http.MethodPut, "/api/v1/alerts/rules/"+formatUUID(rule.ID)+"/enabled", body))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	var resp struct {
		ID              string          `json:"id"`
		Name            string          `json:"name"`
		Enabled         bool            `json:"enabled"`
		ConditionParams json.RawMessage `json:"condition_params"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Enabled {
		t.Error("rule should be disabled after toggle")
	}
}

func TestHandleDeleteAlertRule_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)
	rule, _ := mock.CreateAlertRule(context.Background(), database.CreateAlertRuleParams{
		Name:            "doomed rule",
		Scope:           "global",
		ConditionType:   "agent_offline",
		ConditionParams: []byte(`{"timeout_seconds":300}`),
	})

	req := authedRequest(httptest.NewRequest(http.MethodDelete, "/api/v1/alerts/rules/"+formatUUID(rule.ID), nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", rec.Code)
	}
}

func TestHandleListActiveAlerts_Success(t *testing.T) {
	s, _, _, mock := newTestServer()
	setupTestSession(mock)

	req := authedRequest(httptest.NewRequest(http.MethodGet, "/api/v1/alerts/active", nil))
	rec := httptest.NewRecorder()
	s.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}
