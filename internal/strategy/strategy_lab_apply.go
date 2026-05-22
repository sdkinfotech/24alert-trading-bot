package strategy

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/24alert/trading-bot/pkg/config"
)

// StrategyLabApplyRequest stages or enables a strategy instance.
type StrategyLabApplyRequest struct {
	Phase         string            `json:"phase"` // stage | enable_live (default stage)
	ConfirmLive   bool              `json:"confirm_live"`
	AnalysisVerdict string          `json:"analysis_verdict,omitempty"`
	InstanceID    string            `json:"instance_id"`
	Type          string            `json:"type"`
	AccountID     string            `json:"account_id"`
	InstrumentUID string            `json:"instrument_uid"`
	Ticker        string            `json:"ticker"`
	Params        map[string]string `json:"params"`
	Enabled       bool              `json:"enabled"`
	Start         bool              `json:"start"` // ignored unless enable_live + allow flag
}

// StrategyLabApplyResult reports config write outcome and next rollout steps.
type StrategyLabApplyResult struct {
	Status       string                 `json:"status"`
	Phase        string                 `json:"phase"`
	InstanceID   string                 `json:"instance_id"`
	Reload       map[string]int         `json:"reload"`
	Started      bool                   `json:"started"`
	Rollout      *StrategyLabRolloutPlan `json:"rollout,omitempty"`
	BlockedReasons []string             `json:"blocked_reasons,omitempty"`
}

func (r *Runner) StrategyLabApply(ctx context.Context, req StrategyLabApplyRequest) (*StrategyLabApplyResult, error) {
	phase := strings.TrimSpace(strings.ToLower(req.Phase))
	if phase == "" {
		phase = "stage"
	}
	typ := strings.TrimSpace(req.Type)
	uid := strings.TrimSpace(req.InstrumentUID)
	if typ == "" || uid == "" {
		return nil, fmt.Errorf("type and instrument_uid are required")
	}
	if err := labValidateStrategyType(typ); err != nil {
		return nil, err
	}

	params := req.Params
	if params == nil {
		params = map[string]string{}
	}
	if err := labNormalizeParams(typ, params); err != nil {
		return nil, err
	}

	id := strings.TrimSpace(req.InstanceID)
	if id == "" {
		if cp := r.labConfigProdForUID(uid); cp != nil && cp.InstanceID != "" {
			id = cp.InstanceID
		} else {
			t := strings.TrimSpace(req.Ticker)
			if t == "" {
				t = "inst"
			}
			id = "lab-" + strings.ToLower(t) + "-" + strings.ReplaceAll(typ, "_", "-")
		}
	}

	acct := strings.TrimSpace(req.AccountID)
	if acct == "" {
		if cp := r.labConfigProdForUID(uid); cp != nil && cp.AccountID != "" {
			acct = cp.AccountID
		} else {
			acct = strings.TrimSpace(r.strategiesCfg.ClassicAccountID)
		}
	}
	if acct == "" {
		return nil, fmt.Errorf("account_id is required")
	}

	switch phase {
	case "stage":
		return r.labApplyStage(ctx, id, typ, acct, uid, params)
	case "enable_live":
		return r.labApplyEnableLive(ctx, req, id, typ, acct, uid, params)
	default:
		return nil, fmt.Errorf("unknown phase %q (use stage or enable_live)", phase)
	}
}

func labValidateStrategyType(typ string) error {
	switch strings.ToLower(typ) {
	case "sma_crossover", "sma", "level_bounce":
		return nil
	case "orb_breakout", "ema_1h", "donchian_15m":
		return fmt.Errorf("%s cannot be applied to live runner", typ)
	default:
		return fmt.Errorf("unsupported strategy type %q", typ)
	}
}

func labNormalizeParams(typ string, params map[string]string) error {
	if _, ok := params["quantity"]; !ok {
		params["quantity"] = "1"
	}
	switch strings.ToLower(typ) {
	case "sma_crossover", "sma":
		if _, ok := params["interval"]; !ok {
			params["interval"] = "1h"
		}
		v := strings.TrimSpace(params["trailing_stop_pct"])
		if v == "" || v == "0" {
			return fmt.Errorf("sma_crossover requires trailing_stop_pct > 0 for live trading")
		}
	case "level_bounce":
		if _, ok := params["interval"]; !ok {
			params["interval"] = "15min"
		}
	}
	return nil
}

func (r *Runner) labApplyStage(ctx context.Context, id, typ, acct, uid string, params map[string]string) (*StrategyLabApplyResult, error) {
	inst := config.StrategyInstanceConfig{
		ID:          id,
		Type:        normalizeLabType(typ),
		AccountID:   acct,
		Instruments: []string{uid},
		Enabled:     false,
		Params:      params,
	}
	if err := validateInstanceSafety(inst); err != nil {
		return nil, err
	}
	if err := config.UpsertStrategyInstance(r.configPath, inst); err != nil {
		return nil, err
	}
	added, removed, changed, err := r.ReloadConfig(ctx)
	if err != nil {
		return nil, err
	}
	rollout := buildRolloutPlan(labVerdictDeployCandidate, nil, r.labConfigProdForUID(uid), "ru")
	res := &StrategyLabApplyResult{
		Status:     "staged",
		Phase:      "stage",
		InstanceID: id,
		Reload:     map[string]int{"added": added, "removed": removed, "changed": changed},
		Started:    false,
		Rollout:    &rollout,
	}
	return res, nil
}

func (r *Runner) labApplyEnableLive(ctx context.Context, req StrategyLabApplyRequest, id, typ, acct, uid string, params map[string]string) (*StrategyLabApplyResult, error) {
	var blocked []string
	if !req.ConfirmLive {
		blocked = append(blocked, "confirm_live must be true")
	}
	verdict := strings.TrimSpace(strings.ToLower(req.AnalysisVerdict))
	if verdict != "" && verdict != labVerdictDeployCandidate {
		blocked = append(blocked, fmt.Sprintf("analysis_verdict=%q blocks live enable", verdict))
	}
	if !strategyLabAllowLiveStart() {
		blocked = append(blocked, "STRATEGY_LAB_ALLOW_LIVE_START is not set on strategy-runner")
	}
	if len(blocked) > 0 {
		return &StrategyLabApplyResult{
			Status:         "blocked",
			Phase:          "enable_live",
			InstanceID:     id,
			BlockedReasons: blocked,
		}, nil
	}

	inst := config.StrategyInstanceConfig{
		ID:          id,
		Type:        normalizeLabType(typ),
		AccountID:   acct,
		Instruments: []string{uid},
		Enabled:     true,
		Params:      params,
	}
	if err := validateInstanceSafety(inst); err != nil {
		return nil, err
	}
	if err := config.UpsertStrategyInstance(r.configPath, inst); err != nil {
		return nil, err
	}
	added, removed, changed, err := r.ReloadConfig(ctx)
	if err != nil {
		return nil, err
	}
	res := &StrategyLabApplyResult{
		Status:     "enabled",
		Phase:      "enable_live",
		InstanceID: id,
		Reload:     map[string]int{"added": added, "removed": removed, "changed": changed},
	}
	if req.Start {
		if err := r.StartInstanceByID(ctx, id); err != nil {
			res.Status = "enabled_reload_start_failed: " + err.Error()
			res.Started = false
		} else {
			res.Started = true
			res.Status = "enabled_and_started"
		}
	}
	return res, nil
}

func normalizeLabType(typ string) string {
	if strings.EqualFold(typ, "sma") {
		return "sma_crossover"
	}
	return strings.TrimSpace(typ)
}

func strategyLabAllowLiveStart() bool {
	v := strings.TrimSpace(os.Getenv("STRATEGY_LAB_ALLOW_LIVE_START"))
	return strings.EqualFold(v, "true") || v == "1"
}
