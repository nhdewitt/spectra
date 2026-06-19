package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nhdewitt/spectra/internal/database"
)

const (
	evaluatorInterval  = 60 * time.Second
	diskTrendWindow    = 6 * time.Hour
	diskTrendMinPoints = 5
)

type agentOfflineParams struct {
	TimeoutSeconds int `json:"timeout_seconds"`
}

type diskPredictionParams struct {
	Mount     string `json:"mount"`
	WarnHours int    `json:"warn_hours"`
}

type serviceDownParams struct {
	ServiceName string `json:"service_name"`
}

type agentOfflineSnapshot struct {
	LastSeen      time.Time `json:"last_seen"`
	SecondsSilent int64     `json:"seconds_silent"`
}

type diskPredictionSnapshot struct {
	Mount           string    `json:"mount"`
	UsedPct         float64   `json:"used_pct"`
	PredictedFullAt time.Time `json:"predicted_full_at"`
	HoursRemaining  float64   `json:"hours_remaining"`
}

type serviceDownSnapshot struct {
	ServiceName string `json:"service_name"`
	LastStatus  string `json:"last_status"`
}

// startAlertEvaluator runs the alert evaluation loop in the background.
// It stops when s.done is closed.
func (s *Server) startAlertEvaluator() {
	ticker := time.NewTicker(evaluatorInterval)
	defer ticker.Stop()

	s.Logger.Info("alert evaluator started", "interval", evaluatorInterval)

	for {
		select {
		case <-s.done:
			s.Logger.Info("alert evaluator stopped")
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			s.runEvaluation(ctx)
			cancel()
		}
	}
}

// evalState holds the per-tick data preloaded.
type evalState struct {
	// activeByPair: "ruleID:agentID" -> the open (unresolved) event.
	activeByPair map[string]database.AlertEvent
	// lastByPair: "ruleID:agentID" -> most recent event (resolved or not), for cooldown.
	lastByPair map[string]database.AlertEvent
	// servicesByAgent: agentID string -> that agent's current services.
	servicesByAgent map[string][]database.CurrentService
	servicesLoaded  bool // false if GetAllServices failed this tick
}

func pairKey(ruleID, agentID pgtype.UUID) string {
	return formatUUID(ruleID) + ":" + formatUUID(agentID)
}

func (s *Server) runEvaluation(ctx context.Context) {
	rules, err := s.DB.ListEnabledAlertRules(ctx)
	if err != nil {
		s.Logger.Error("alert evaluator: list rules", "err", err)
		return
	}

	agents, err := s.DB.ListAgents(ctx)
	if err != nil {
		s.Logger.Error("alert evaluator: list agents", "err", err)
		return
	}

	agentByID := make(map[string]database.ListAgentsRow, len(agents))
	for _, a := range agents {
		agentByID[formatUUID(a.ID)] = a
	}

	ruleChannels := make(map[string][]database.AlertChannel)
	for _, rule := range rules {
		key := formatUUID(rule.ID)
		if _, loaded := ruleChannels[key]; loaded {
			continue
		}
		channels, err := s.DB.ListChannelsForRule(ctx, rule.ID)
		if err != nil {
			s.Logger.Warn("alert evaluator: list channels for rule", "rule", rule.Name, "err", err)
			ruleChannels[key] = nil
			continue
		}
		ruleChannels[key] = channels
	}

	state := s.preloadEvalState(ctx, rules)

	agentRulesEvaluated := make(map[string]bool)

	for _, rule := range rules {
		if rule.Scope != "agent" || !rule.AgentID.Valid {
			continue
		}
		agent, ok := agentByID[formatUUID(rule.AgentID)]
		if !ok {
			// Rule references an agent that no longer exists. Skip.
			continue
		}
		channels := ruleChannels[formatUUID(rule.ID)]
		s.evaluateRuleForAgent(ctx, rule, agent, channels, state)
		key := fmt.Sprintf("%s:%s", formatUUID(rule.AgentID), rule.ConditionType)
		agentRulesEvaluated[key] = true
	}

	for _, rule := range rules {
		if rule.Scope != "global" {
			continue
		}
		channels := ruleChannels[formatUUID(rule.ID)]
		for _, agent := range agents {
			skipKey := fmt.Sprintf("%s:%s", formatUUID(agent.ID), rule.ConditionType)
			if agentRulesEvaluated[skipKey] {
				continue
			}
			s.evaluateRuleForAgent(ctx, rule, agent, channels, state)
		}
	}
}

// preloadEvalState issues the bulk queries that back per-pair lookups. Failures
// degrade gracefully: a nil/empty map just means the evaluator treats those lookups
// as misses (no active event/no cooldown/no services).
func (s *Server) preloadEvalState(ctx context.Context, rules []database.AlertRule) *evalState {
	state := &evalState{
		activeByPair:    make(map[string]database.AlertEvent),
		lastByPair:      make(map[string]database.AlertEvent),
		servicesByAgent: make(map[string][]database.CurrentService),
	}

	active, err := s.DB.ListAllActiveEvents(ctx)
	if err != nil {
		s.Logger.Error("alert evaluator: preload active events", "err", err)
	} else {
		for _, ev := range active {
			state.activeByPair[pairKey(ev.RuleID, ev.AgentID)] = ev
		}
	}

	last, err := s.DB.ListLastEventPerRuleAgent(ctx)
	if err != nil {
		s.Logger.Error("alert evaluator: preload last events", "err", err)
	} else {
		for _, ev := range last {
			state.lastByPair[pairKey(ev.RuleID, ev.AgentID)] = ev
		}
	}

	// Only load services if at least one enabled rule needs them
	needServices := false
	for _, r := range rules {
		if r.ConditionType == "service_down" {
			needServices = true
			break
		}
	}
	if needServices {
		svcs, err := s.DB.GetAllServices(ctx)
		if err != nil {
			s.Logger.Error("alert evaluator: preload services", "err", err)
		} else {
			state.servicesLoaded = true
			for _, svc := range svcs {
				key := formatUUID(svc.AgentID)
				state.servicesByAgent[key] = append(state.servicesByAgent[key], svc)
			}
		}
	}

	return state
}

// evaluateRuleForAgent evaluates one rule against one agent.
func (s *Server) evaluateRuleForAgent(ctx context.Context, rule database.AlertRule, agent database.ListAgentsRow, channels []database.AlertChannel, state *evalState) {
	fired, snapshot, err := s.checkCondition(ctx, rule, agent, state)
	if err != nil {
		s.Logger.Warn("alert evaluator: check condition", "rule", rule.Name, "agent", formatUUID(agent.ID), "err", err)
		return
	}

	key := pairKey(rule.ID, agent.ID)
	_, hasActive := state.activeByPair[key]

	switch {
	case fired && !hasActive:
		// Cooldown: most recent event from preloaded state (resolved or not)
		if last, ok := state.lastByPair[key]; ok {
			cooldown := time.Duration(rule.CooldownSeconds) * time.Second
			if time.Since(last.FiredAt.Time) < cooldown {
				return
			}
		}
		ev, err := s.DB.CreateAlertEvent(ctx, database.CreateAlertEventParams{
			RuleID:            rule.ID,
			AgentID:           agent.ID,
			ConditionSnapshot: mustMarshal(snapshot),
		})
		if err != nil {
			if isPgUniqueViolation(err) {
				return
			}
			s.Logger.Error("alert evaluator: create event", "rule", rule.Name, "err", err)
			return
		}
		state.activeByPair[key] = ev
		state.lastByPair[key] = ev
		s.Logger.Info("alert fired",
			"rule", rule.Name,
			"agent", formatUUID(agent.ID),
			"condition", rule.ConditionType,
		)
		s.notifyAsync(ev, rule, channels, snapshot)

	case !fired && hasActive:
		if err := s.DB.ResolveAlertEvent(ctx, database.ResolveAlertEventParams{
			RuleID:  rule.ID,
			AgentID: agent.ID,
		}); err != nil {
			s.Logger.Error("alert evaluator: resolve event", "rule", rule.Name, "err", err)
			return
		}
		delete(state.activeByPair, key)
		s.Logger.Info("alert resolved", "rule", rule.Name, "agent", formatUUID(agent.ID))

	case fired && hasActive:
		// Already firing
	}
}

// checkCondition evaluates the rule condition and returns whether it is
// currently firing plus a snapshot value for storage.
func (s *Server) checkCondition(ctx context.Context, rule database.AlertRule, agent database.ListAgentsRow, state *evalState) (bool, any, error) {
	switch rule.ConditionType {
	case "agent_offline":
		return s.checkAgentOffline(rule, agent)
	case "disk_prediction":
		return s.checkDiskPrediction(ctx, rule, agent.ID)
	case "service_down":
		if !state.servicesLoaded {
			// can't evaluate
			return false, nil, nil
		}
		return checkServiceDown(rule, state.servicesByAgent[formatUUID(agent.ID)])
	default:
		return false, nil, fmt.Errorf("unknown condition type %q", rule.ConditionType)
	}
}

func (s *Server) checkAgentOffline(rule database.AlertRule, agent database.ListAgentsRow) (bool, any, error) {
	var params agentOfflineParams
	if err := json.Unmarshal(rule.ConditionParams, &params); err != nil {
		return false, nil, fmt.Errorf("parse params: %w", err)
	}
	if params.TimeoutSeconds <= 0 {
		params.TimeoutSeconds = 300
	}

	if !agent.LastSeen.Valid {
		// Never reported in, treat as offline if a timeout is configured
		return true, agentOfflineSnapshot{SecondsSilent: -1}, nil
	}

	silent := time.Since(agent.LastSeen.Time)
	threshold := time.Duration(params.TimeoutSeconds) * time.Second

	if silent < threshold {
		return false, nil, nil
	}

	snap := agentOfflineSnapshot{
		LastSeen:      agent.LastSeen.Time,
		SecondsSilent: int64(silent.Seconds()),
	}
	return true, snap, nil
}

func (s *Server) checkDiskPrediction(ctx context.Context, rule database.AlertRule, agentID pgtype.UUID) (bool, any, error) {
	var params diskPredictionParams
	if err := json.Unmarshal(rule.ConditionParams, &params); err != nil {
		return false, nil, fmt.Errorf("parse params: %w", err)
	}
	if params.WarnHours <= 0 {
		params.WarnHours = 72
	}

	rows, err := s.DB.GetDiskTrend(ctx, database.GetDiskTrendParams{
		AgentID:    agentID,
		Mountpoint: pgText(params.Mount),
		StartTime:  pgtype.Timestamptz{Time: time.Now().Add(-diskTrendWindow), Valid: true},
	})
	if err != nil {
		return false, nil, err
	}
	if len(rows) < diskTrendMinPoints {
		return false, nil, nil // not enough data
	}

	hoursRemaining, currentPct, err := linearProjectHours(rows)
	if err != nil {
		return false, nil, err
	}
	if hoursRemaining < 0 {
		// Flat or shrinking, don't fire
		return false, nil, nil
	}

	if hoursRemaining > float64(params.WarnHours) {
		return false, nil, nil
	}

	snap := diskPredictionSnapshot{
		Mount:           params.Mount,
		UsedPct:         currentPct,
		PredictedFullAt: time.Now().Add(time.Duration(hoursRemaining * float64(time.Hour))),
		HoursRemaining:  hoursRemaining,
	}
	return true, snap, nil
}

func checkServiceDown(rule database.AlertRule, services []database.CurrentService) (bool, any, error) {
	var params serviceDownParams
	if err := json.Unmarshal(rule.ConditionParams, &params); err != nil {
		return false, nil, fmt.Errorf("parse params: %w", err)
	}

	for _, svc := range services {
		if !strings.EqualFold(svc.Name, params.ServiceName) {
			continue
		}
		status := ""
		if svc.Status.Valid {
			status = svc.Status.String
		}
		if serviceStatusHealthy(status) {
			return false, nil, nil
		}
		snap := serviceDownSnapshot{
			ServiceName: svc.Name,
			LastStatus:  status,
		}
		return true, snap, nil
	}

	// Service not found in list - treat as down
	snap := serviceDownSnapshot{
		ServiceName: params.ServiceName,
		LastStatus:  "not found",
	}
	return true, snap, nil
}

// serviceStatusHealthy reports whether a collector's Status value represents a
// healthy/up service. The vocabulary differs per platform collector:
//
//	systemd: 	"active" (SubStatus carries running/dead/exited)
//	launchd: 	"running"
//	rc.d:		"active" for enabled services. The FreeBSD collector
//				reports enabled-state, not live process state, so a
//				crashed-but-enabled service still reads "active".
//				service_down alerts on FreeBSD therefore only catch
//				a service being disabled, not one that has crashed.
//	SC:			"Running". Transitional states (StartPending/
//				StopPending/ContinuePending/PausePending) are treated
//				as healthy so a normal service restart doesn't produce
//				a brief false-positive that auto-resolves on the next
//				tick. "Stopped"/"Paused" are genuinely down.
//
// Matching is case-insensitive to handle "Running" from the Windows collector.
func serviceStatusHealthy(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active", "running":
		return true
	case "startpending", "stoppending", "continuepending", "pausepending":
		return true
	default:
		return false
	}
}

// linearProjectHours fits a least-squares line to disk usage samples and
// returns the projected hours until 100% usage along with the latest observed
// usage percentage. Returns hoursRemaining = -1 when the disk is not filling
// (slope <= 0). Only samples with valid time and usage are considered.
func linearProjectHours(rows []database.GetDiskTrendRow) (hoursRemaining, latestPct float64, err error) {
	type point struct{ x, y float64 }
	var pts []point
	var t0 time.Time

	for _, r := range rows {
		if !r.Time.Valid || !r.UsedPercent.Valid {
			continue
		}
		if len(pts) == 0 {
			t0 = r.Time.Time
		}
		pts = append(pts, point{
			x: r.Time.Time.Sub(t0).Hours(),
			y: r.UsedPercent.Float64,
		})
	}

	if len(pts) < 2 {
		return 0, 0, fmt.Errorf("insufficient valid data points")
	}

	n := float64(len(pts))
	var sumX, sumY, sumXY, sumX2 float64
	for _, p := range pts {
		sumX += p.x
		sumY += p.y
		sumXY += p.x * p.y
		sumX2 += p.x * p.x
	}

	denom := n*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-10 {
		return 0, 0, fmt.Errorf("degenerate regression (all samples at same time)")
	}

	slope := (n*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / n
	if math.IsNaN(slope) || math.IsInf(slope, 0) {
		return 0, 0, fmt.Errorf("non-finite slope")
	}

	latest := pts[len(pts)-1]
	latestPct = latest.y // actual last reading

	if slope <= 0 {
		return -1, latestPct, nil
	}

	xFull := (100.0 - intercept) / slope
	hoursRemaining = xFull - latest.x

	return hoursRemaining, latestPct, nil
}

// notifyTimeout bounds a single channel delivery on the detached notification path.
// The async notifier is not tied to the evaluator tick's context - a send must not
// be cancelled when the tick returns - so it establishes its own context with this
// timeout. Lower-level send functions stay context-aware and never choose their own
// lifecycle.
const notifyTimeout = 15 * time.Second

// notifyAsync delivers alert notifications on a goroutine using the pre-loaded
// channel list.
func (s *Server) notifyAsync(ev database.AlertEvent, rule database.AlertRule, channels []database.AlertChannel, snapshot any) {
	go func() {
		for _, ch := range channels {
			ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
			err := s.sendNotification(ctx, ch, rule, ev, snapshot)
			cancel()

			if err != nil {
				s.Logger.Error("notifier: send", "channel", ch.Name, "type", ch.Type, "rule", rule.Name, "err", err)
				continue
			}

			tctx, tcancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = s.DB.TouchAlertEventNotified(tctx, ev.ID)
			tcancel()
		}
	}()
}

// AlertPayload is the JSON body sent to webhook channels.
type AlertPayload struct {
	EventID       string `json:"event_id"`
	RuleID        string `json:"rule_id"`
	RuleName      string `json:"rule_name"`
	ConditionType string `json:"condition_type"`
	AgentID       string `json:"agent_id"`
	FiredAt       string `json:"fired_at"`
	Snapshot      any    `json:"snapshot"`
}

func (s *Server) sendNotification(ctx context.Context, ch database.AlertChannel, rule database.AlertRule, ev database.AlertEvent, snapshot any) error {
	payload := AlertPayload{
		EventID:       formatUUID(ev.ID),
		RuleID:        formatUUID(rule.ID),
		RuleName:      rule.Name,
		ConditionType: rule.ConditionType,
		AgentID:       formatUUID(ev.AgentID),
		FiredAt:       ev.FiredAt.Time.UTC().Format(time.RFC3339),
		Snapshot:      snapshot,
	}

	switch ch.Type {
	case "webhook":
		return s.sendWebhook(ctx, ch, payload)
	case "email":
		return s.sendEmail(ctx, ch, payload)
	default:
		return fmt.Errorf("unknown channel type %q", ch.Type)
	}
}

func (s *Server) sendWebhook(ctx context.Context, ch database.AlertChannel, payload AlertPayload) error {
	var cfg struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(ch.Config, &cfg); err != nil {
		return fmt.Errorf("parse webhook config: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, cfg.URL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Spectra-Alerting/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}
