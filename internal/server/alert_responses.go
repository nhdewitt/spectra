package server

import (
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

// The database row types store the JSON columns config, condition_params,
// condition_snapshot as []byte. encoding/json marshals []byte as base64, so
// returning the raw rows would put base64-encoded JSON on the wire. These
// response structs re-type those fields as json.RawMessage so the columns are
// emitted as native JSON. pgtype.UUID/pgtype.Timestamptz are already marshalled
// correctly, so they pass through.

// channelResponse is the wire shape for an alert channel.
type channelResponse struct {
	ID        pgtype.UUID        `json:"id"`
	Name      string             `json:"name"`
	Type      string             `json:"type"`
	Config    json.RawMessage    `json:"config"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

func toChannelResponse(c database.AlertChannel) channelResponse {
	return channelResponse{
		ID:        c.ID,
		Name:      c.Name,
		Type:      c.Type,
		Config:    json.RawMessage(c.Config),
		CreatedAt: c.CreatedAt,
	}
}

func toChannelResponses(cs []database.AlertChannel) []channelResponse {
	out := make([]channelResponse, len(cs))
	for i, c := range cs {
		out[i] = toChannelResponse(c)
	}
	return out
}

// ruleView is the wire shape for an alert rule, with condition_params emitted as
// native JSON rather than base64.
type ruleView struct {
	ID              pgtype.UUID        `json:"id"`
	Name            string             `json:"name"`
	Enabled         bool               `json:"enabled"`
	Scope           string             `json:"scope"`
	AgentID         pgtype.UUID        `json:"agent_id"`
	ConditionType   string             `json:"condition_type"`
	ConditionParams json.RawMessage    `json:"condition_params"`
	CooldownSeconds int32              `json:"cooldown_seconds"`
	CreatedAt       pgtype.Timestamptz `json:"created_at"`
	UpdatedAt       pgtype.Timestamptz `json:"updated_at"`
}

func toRuleView(r database.AlertRule) ruleView {
	return ruleView{
		ID:              r.ID,
		Name:            r.Name,
		Enabled:         r.Enabled,
		Scope:           r.Scope,
		AgentID:         r.AgentID,
		ConditionType:   r.ConditionType,
		ConditionParams: json.RawMessage(r.ConditionParams),
		CooldownSeconds: r.CooldownSeconds,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func toRuleViews(rs []database.AlertRule) []ruleView {
	out := make([]ruleView, len(rs))
	for i, r := range rs {
		out[i] = toRuleView(r)
	}
	return out
}

// eventView is the wire shape for an alert event row from the joined
// active/history queries. condition_snapshot is emitted as native JSON; the
// joined rule_name/condition_type/hostname are denormalized for display.
// Hostname is omitempty because the per-agent history query does not join it
// (the agent is already implied by the route).
type eventView struct {
	ID                pgtype.UUID        `json:"id"`
	RuleID            pgtype.UUID        `json:"rule_id"`
	AgentID           pgtype.UUID        `json:"agent_id"`
	FiredAt           pgtype.Timestamptz `json:"fired_at"`
	ResolvedAt        pgtype.Timestamptz `json:"resolved_at"`
	LastNotifiedAt    pgtype.Timestamptz `json:"last_notified_at"`
	ConditionSnapshot json.RawMessage    `json:"condition_snapshot"`
	RuleName          string             `json:"rule_name"`
	ConditionType     string             `json:"condition_type"`
	Hostname          string             `json:"hostname,omitempty"`
}

func toActiveEventViews(rows []database.ListActiveAlertEventsRow) []eventView {
	out := make([]eventView, len(rows))
	for i, e := range rows {
		out[i] = eventView{
			ID:                e.ID,
			RuleID:            e.RuleID,
			AgentID:           e.AgentID,
			FiredAt:           e.FiredAt,
			ResolvedAt:        e.ResolvedAt,
			LastNotifiedAt:    e.LastNotifiedAt,
			ConditionSnapshot: json.RawMessage(e.ConditionSnapshot),
			RuleName:          e.RuleName,
			ConditionType:     e.ConditionType,
			Hostname:          e.Hostname,
		}
	}
	return out
}

func toHistoryEventViews(rows []database.ListAlertEventHistoryRow) []eventView {
	out := make([]eventView, len(rows))
	for i, e := range rows {
		out[i] = eventView{
			ID:                e.ID,
			RuleID:            e.RuleID,
			AgentID:           e.AgentID,
			FiredAt:           e.FiredAt,
			ResolvedAt:        e.ResolvedAt,
			LastNotifiedAt:    e.LastNotifiedAt,
			ConditionSnapshot: json.RawMessage(e.ConditionSnapshot),
			RuleName:          e.RuleName,
			ConditionType:     e.ConditionType,
			Hostname:          e.Hostname,
		}
	}
	return out
}

func toAgentEventViews(rows []database.ListAlertEventsByAgentRow) []eventView {
	out := make([]eventView, len(rows))
	for i, e := range rows {
		out[i] = eventView{
			ID:                e.ID,
			RuleID:            e.RuleID,
			AgentID:           e.AgentID,
			FiredAt:           e.FiredAt,
			ResolvedAt:        e.ResolvedAt,
			LastNotifiedAt:    e.LastNotifiedAt,
			ConditionSnapshot: json.RawMessage(e.ConditionSnapshot),
			RuleName:          e.RuleName,
			ConditionType:     e.ConditionType,
			// No Hostname — the by-agent query does not join agents.
		}
	}
	return out
}
