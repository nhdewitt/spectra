package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

const (
	defaultAlertHistoryLimit = 50
	maxAlertHistoryLimit     = 200
	maxAlertHistoryOffset    = 100_000
)

type channelRequest struct {
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

func validateChannelRequest(req channelRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}
	if len(req.Config) == 0 || !json.Valid(req.Config) {
		return errors.New("config must be valid JSON")
	}

	switch req.Type {
	case "webhook":
		var c struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(req.Config, &c); err != nil || strings.TrimSpace(c.URL) == "" {
			return errors.New("webhook config requires a non-empty url")
		}
	case "email":
		var c struct {
			To string `json:"to"`
		}
		if err := json.Unmarshal(req.Config, &c); err != nil || strings.TrimSpace(c.To) == "" {
			return errors.New("email config requires a non-empty to address")
		}
	default:
		return fmt.Errorf("invalid channel type %q (must be email or webhook)", req.Type)
	}

	return nil
}

// handleListAlertChannels returns all configured alert channels.
//
// GET /api/v1/alerts/channels
func (s *Server) handleListAlertChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := s.DB.ListAlertChannels(r.Context())
	if err != nil {
		s.dbError(w, err, "handleListAlertChannels")
		return
	}
	respondJSON(w, http.StatusOK, toChannelResponses(channels))
}

// handleCreateAlertChannel creates a new alert channel.
//
// POST /api/v1/alerts/channels
func (s *Server) handleCreateAlertChannel(w http.ResponseWriter, r *http.Request) {
	var req channelRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateChannelRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ch, err := s.DB.CreateAlertChannel(r.Context(), database.CreateAlertChannelParams{
		Name:   req.Name,
		Type:   req.Type,
		Config: req.Config,
	})
	if err != nil {
		s.dbError(w, err, "handleCreateAlertChannel")
		return
	}

	s.Logger.Info("alert channel created", "channel_id", formatUUID(ch.ID), "type", ch.Type)
	respondJSON(w, http.StatusCreated, toChannelResponse(ch))
}

// handleUpdateAlertChannel updates an existing alert channel.
//
// PUT /api/v1/alerts/channels/{id}
func (s *Server) handleUpdateAlertChannel(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req channelRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateChannelRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ch, err := s.DB.UpdateAlertChannel(r.Context(), database.UpdateAlertChannelParams{
		ID:     mustUUID(id),
		Name:   req.Name,
		Type:   req.Type,
		Config: req.Config,
	})
	if err != nil {
		s.dbError(w, err, "handleUpdateAlertChannel")
		return
	}

	s.Logger.Info("alert channel updated", "channel_id", id)
	respondJSON(w, http.StatusOK, toChannelResponse(ch))
}

// handleDeleteAlertChannel deletes an alert channel. The alert_rule_channels
// rows referencing it are removed by ON DELETE CASCADE.
//
// DELETE /api/v1/alerts/channels/{id}
func (s *Server) handleDeleteAlertChannel(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.DB.DeleteAlertChannel(r.Context(), mustUUID(id)); err != nil {
		s.dbError(w, err, "handleDeleteAlertChannel")
		return
	}

	s.Logger.Info("alert channel deleted", "channel_id", id)
	w.WriteHeader(http.StatusNoContent)
}

type ruleRequest struct {
	Name            string          `json:"name"`
	Enabled         bool            `json:"enabled"`
	Scope           string          `json:"scope"`
	AgentID         string          `json:"agent_id"`
	ConditionType   string          `json:"condition_type"`
	ConditionParams json.RawMessage `json:"condition_params"`
	CooldownSeconds int32           `json:"cooldown_seconds"`
	ChannelIDs      []string        `json:"channel_ids"`
}

// ruleResponse wraps the created/updated rule plus any non-fatal warnings
// (e.g. service_down targeting a service not currently reported).
type ruleResponse struct {
	Rule     ruleView `json:"rule"`
	Warnings []string `json:"warnings,omitempty"`
}

func validateRuleRequest(req ruleRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("name is required")
	}
	switch req.Scope {
	case "global":
		if strings.TrimSpace(req.AgentID) != "" {
			return errors.New("global rules must not specify an agent_id")
		}
	case "agent":
		if !uuidRegex.MatchString(req.AgentID) {
			return errors.New("agent rules require a valid agent_id")
		}
	default:
		return fmt.Errorf("invalid scope %q (must be global or agent)", req.Scope)
	}

	switch req.ConditionType {
	case "agent_offline", "disk_prediction", "service_down":
	default:
		return fmt.Errorf("invalid condition_type %q", req.ConditionType)
	}

	// service_down is inherently agent-scoped. A global service rule would fire
	// on every agent not running the service
	if req.ConditionType == "service_down" && req.Scope == "global" {
		return errors.New("service_down rules must target a specific agent")
	}

	if len(req.ConditionParams) == 0 || !json.Valid(req.ConditionParams) {
		return errors.New("condition_params must be valid JSON")
	}
	if req.CooldownSeconds < 0 {
		return errors.New("cooldown_seconds must not be negative")
	}
	for _, cid := range req.ChannelIDs {
		if !uuidRegex.MatchString(cid) {
			return fmt.Errorf("invalid channel id %q", cid)
		}
	}
	return nil
}

// serviceDownWarnings returns a warning if an agent-scoped service_down rule
// targets a service the agent isn't currently reporting. Non-fatal: the rule is
// still created (the service may simply be installed later).
func (s *Server) serviceDownWarnings(r *http.Request, req ruleRequest) []string {
	if req.ConditionType != "service_down" || req.Scope != "agent" {
		return nil
	}
	var p serviceDownParams
	if err := json.Unmarshal(req.ConditionParams, &p); err != nil || p.ServiceName == "" {
		return nil // validation happens elsewhere
	}

	services, err := s.DB.GetServices(r.Context(), mustUUID(req.AgentID))
	if err != nil {
		return nil // don't block creation on a transient read error
	}
	for _, svc := range services {
		if strings.EqualFold(svc.Name, p.ServiceName) {
			return nil
		}
	}

	return []string{
		fmt.Sprintf("service %q is not currently reported by this agent; the rule will fire until the service appears",
			p.ServiceName,
		),
	}
}

// setRuleChannels replaces a rule's channel associations with the given set,
// inside a best-effort sequence (delete-all then add-each). Invalid or unknown
// channel IDs that fail the FK are surfaced as an error.
func (s *Server) setRuleChannels(r *http.Request, ruleID pgtype.UUID, channelIDs []string) error {
	if err := s.DB.DeleteChannelsForRule(r.Context(), ruleID); err != nil {
		return err
	}
	for _, cid := range channelIDs {
		if err := s.DB.AddChannelToRule(r.Context(), database.AddChannelToRuleParams{
			RuleID:    ruleID,
			ChannelID: mustUUID(cid),
		}); err != nil {
			return err
		}
	}
	return nil
}

// handleListAlertRules returns all alert rules.
//
// GET /api/v1/alerts/rules
func (s *Server) handleListAlertRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.DB.ListAlertRules(r.Context())
	if err != nil {
		s.dbError(w, err, "handleListAlertRules")
		return
	}
	respondJSON(w, http.StatusOK, toRuleViews(rules))
}

// handleGetAlertRule returns a single rule plus its channel associations.
//
// GET /api/v1/alerts/rules/{id}
func (s *Server) handleGetAlertRule(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rule, err := s.DB.GetAlertRule(r.Context(), mustUUID(id))
	if err != nil {
		http.Error(w, "rule not found", http.StatusNotFound)
		return
	}
	channels, err := s.DB.ListChannelsForRule(r.Context(), mustUUID(id))
	if err != nil {
		s.dbError(w, err, "handleGetAlertRule")
		return
	}

	type resp struct {
		Rule     ruleView          `json:"rule"`
		Channels []channelResponse `json:"channels"`
	}
	respondJSON(w, http.StatusOK, resp{
		Rule:     toRuleView(rule),
		Channels: toChannelResponses(channels),
	})
}

// handleCreateAlertRule creates a rule and attaches its channels.
//
// POST /api/v1/alerts/rules
func (s *Server) handleCreateAlertRule(w http.ResponseWriter, r *http.Request) {
	var req ruleRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateRuleRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var agentID pgtype.UUID
	if req.Scope == "agent" {
		agentID = mustUUID(req.AgentID)
	}

	rule, err := s.DB.CreateAlertRule(r.Context(), database.CreateAlertRuleParams{
		Name:            req.Name,
		Enabled:         req.Enabled,
		Scope:           req.Scope,
		AgentID:         agentID,
		ConditionType:   req.ConditionType,
		ConditionParams: req.ConditionParams,
		CooldownSeconds: req.CooldownSeconds,
	})
	if err != nil {
		s.dbError(w, err, "handleCreateAlertRule")
		return
	}

	if err := s.setRuleChannels(r, rule.ID, req.ChannelIDs); err != nil {
		// rule exists but channel wiring failed - report so the caller can retry
		s.Logger.Error("failed to set rule channels", "error", err, "rule_id", formatUUID(rule.ID))
		http.Error(w, "rule created but channel association failed", http.StatusInternalServerError)
		return
	}

	s.Logger.Info("alert rule created",
		"rule_id", formatUUID(rule.ID), "scope", rule.Scope, "condition", rule.ConditionType)
	respondJSON(w, http.StatusCreated, ruleResponse{
		Rule:     toRuleView(rule),
		Warnings: s.serviceDownWarnings(r, req),
	})
}

// handleUpdateAlertRule updates a rule's mutable fields and re-syncs channels.
// Scope, agent, and condition_type are immutable post-creation.
//
// PUT /api/v1/alerts/rules/{id}
func (s *Server) handleUpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	existing, err := s.DB.GetAlertRule(r.Context(), mustUUID(id))
	if err != nil {
		http.Error(w, "rule not found", http.StatusNotFound)
		return
	}

	var req ruleRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// preserve immutable fields from the existing rule
	req.Scope = existing.Scope
	req.ConditionType = existing.ConditionType
	req.AgentID = formatUUID(existing.AgentID)
	if existing.Scope == "global" {
		req.AgentID = ""
	}
	if err := validateRuleRequest(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updated, err := s.DB.UpdateAlertRule(r.Context(), database.UpdateAlertRuleParams{
		ID:              mustUUID(id),
		Name:            req.Name,
		Enabled:         req.Enabled,
		ConditionParams: req.ConditionParams,
		CooldownSeconds: req.CooldownSeconds,
	})
	if err != nil {
		s.dbError(w, err, "handleUpdateAlertRule")
		return
	}

	if err := s.setRuleChannels(r, updated.ID, req.ChannelIDs); err != nil {
		s.Logger.Error("failed to set rule channels", "error", err, "rule_id", id)
		http.Error(w, "rule updated but channel association failed", http.StatusInternalServerError)
		return
	}

	s.Logger.Info("alert rule updated", "rule_id", id)
	respondJSON(w, http.StatusOK, ruleResponse{
		Rule:     toRuleView(updated),
		Warnings: s.serviceDownWarnings(r, req),
	})
}

// handleSetAlertRuleEnabled toggles a rule's enabled flag.
//
// PUT /api/v1/alerts/rules/{id}/enabled
func (s *Server) handleSetAlertRuleEnabled(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rule, err := s.DB.SetAlertRuleEnabled(r.Context(), database.SetAlertRuleEnabledParams{
		ID:      mustUUID(id),
		Enabled: req.Enabled,
	})
	if err != nil {
		s.dbError(w, err, "handleSetAlertRuleEnabled")
		return
	}

	s.Logger.Info("alert rule enabled toggled", "rule_id", id, "enabled", req.Enabled)
	respondJSON(w, http.StatusOK, toRuleView(rule))
}

// handleDeleteAlertRule deletes a rule. Channel associations and events are
// removed by ON DELETE CASCADE.
//
// DELETE /api/v1/alerts/rules/{id}
func (s *Server) handleDeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.DB.DeleteAlertRule(r.Context(), mustUUID(id)); err != nil {
		s.dbError(w, err, "handleDeleteAlertRule")
		return
	}

	s.Logger.Info("alert rule deleted", "rule_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// handleListActiveAlerts returns all currently firing (unresolved) alerts.
//
// GET /api/v1/alerts/active
func (s *Server) handleListActiveAlerts(w http.ResponseWriter, r *http.Request) {
	events, err := s.DB.ListActiveAlertEvents(r.Context())
	if err != nil {
		s.dbError(w, err, "handleListActiveAlerts")
		return
	}
	respondJSON(w, http.StatusOK, toActiveEventViews(events))
}

// parseLimitOffset reads limit/offset query params with defaults and bounds.
// limit defaults to defaultAlertHistoryLimit and is capped at maxAlertHistoryLimit;
// offset defaults to 0. Returns an error for malformed or out-of-range values.
func parseLimitOffset(r *http.Request) (limit, offset int32, err error) {
	limit = defaultAlertHistoryLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		n, convErr := strconv.Atoi(v)
		if convErr != nil || n < 1 {
			return 0, 0, errors.New("limit must be a positive integer")
		}
		if n > maxAlertHistoryLimit {
			return 0, 0, fmt.Errorf("limit must not exceed %d", maxAlertHistoryLimit)
		}
		limit = int32(n)
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, convErr := strconv.Atoi(v)
		if convErr != nil || n < 0 {
			return 0, 0, errors.New("offset must be a non-negative integer")
		}
		if n > maxAlertHistoryOffset {
			return 0, 0, fmt.Errorf("offset must not exceed %d", maxAlertHistoryOffset)
		}
		offset = int32(n)
	}
	return limit, offset, nil
}

// handleListAlertHistory returns paginated alert event history for all agents.
//
// GET /api/v1/alerts/history?limit=&offset=
func (s *Server) handleListAlertHistory(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parseLimitOffset(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	events, err := s.DB.ListAlertEventHistory(r.Context(), database.ListAlertEventHistoryParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		s.dbError(w, err, "handleListAlertHistory")
		return
	}
	respondJSON(w, http.StatusOK, toHistoryEventViews(events))
}

// handleListAgentAlertHistory returns paginated alert event history for one agent.
//
// GET /api/v1/agents/{id}/alerts/history?limit=&offset=
func (s *Server) handleListAgentAlertHistory(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	limit, offset, err := parseLimitOffset(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	events, err := s.DB.ListAlertEventsByAgent(r.Context(), database.ListAlertEventsByAgentParams{
		AgentID: mustUUID(id),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		s.dbError(w, err, "handleListAgentAlertHistory")
		return
	}
	respondJSON(w, http.StatusOK, toAgentEventViews(events))
}
