package strategy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/24alert/trading-bot/pkg/config"
)

// StrategyLabCatalog describes available backtest families for the UI.
type StrategyLabCatalog struct {
	Strategies []StrategyLabStrategyMeta `json:"strategies"`
	CoreTickers []StrategyLabTickerMeta `json:"core_tickers"`
}

type StrategyLabStrategyMeta struct {
	ID            string   `json:"id"`
	Label         string   `json:"label"`
	Interval      string   `json:"interval"`
	LiveEligible  bool     `json:"live_eligible"`
	LiveBlockNote string   `json:"live_block_note,omitempty"`
	ParamHints    []string `json:"param_hints"`
}

type StrategyLabTickerMeta struct {
	Ticker string `json:"ticker"`
	UID    string `json:"uid"`
}

type StrategyLabCompareRequest struct {
	UID    string `json:"uid"`
	Ticker string `json:"ticker"`
	Days   int    `json:"days"`
}

type StrategyLabOptimizeRequest struct {
	UID      string `json:"uid"`
	Ticker   string `json:"ticker"`
	Strategy string `json:"strategy"`
	Days     int    `json:"days"`
}

type StrategyLabApplyRequest struct {
	InstanceID    string            `json:"instance_id"`
	Type          string            `json:"type"`
	AccountID     string            `json:"account_id"`
	InstrumentUID string            `json:"instrument_uid"`
	Ticker        string            `json:"ticker"`
	Params        map[string]string `json:"params"`
	Enabled       bool              `json:"enabled"`
	Start         bool              `json:"start"`
}

type StrategyLabApplyResult struct {
	Status   string `json:"status"`
	InstanceID string `json:"instance_id"`
	Reload   map[string]int `json:"reload"`
	Started  bool   `json:"started"`
}

func strategyLabCatalog() StrategyLabCatalog {
	return StrategyLabCatalog{
		CoreTickers: []StrategyLabTickerMeta{
			{Ticker: "BMM6", UID: "dc1ffa30-70a4-4a7b-807a-4f31c2951f7e"},
			{Ticker: "NGM6", UID: "117a1408-431f-4ba0-a041-5bba3123d4a8"},
			{Ticker: "MCM6", UID: "6f4563c0-e853-46f2-98c7-3abce3cc7517"},
		},
		Strategies: []StrategyLabStrategyMeta{
			{
				ID: "sma_crossover", Label: "SMA Crossover", Interval: "1h",
				LiveEligible: true,
				ParamHints: []string{"fast_period", "slow_period", "trailing_stop_pct", "initial_stop_swing_bars", "quantity"},
			},
			{
				ID: "level_bounce", Label: "Level Bounce", Interval: "15min",
				LiveEligible: true,
				ParamHints: []string{"sl_mult", "tp_mult", "cutoff_hour", "cutoff_min", "level_days", "quantity"},
			},
			{
				ID: "orb_breakout", Label: "ORB Breakout", Interval: "15min",
				LiveEligible: false, LiveBlockNote: "blocked on live runner until protective stop",
				ParamHints: []string{"range_candles", "cutoff_hour", "cutoff_min", "quantity"},
			},
			{
				ID: "ema_1h", Label: "EMA Crossover (research)", Interval: "1h",
				LiveEligible: false, LiveBlockNote: "not implemented in Go runner",
				ParamHints: []string{"fast_period", "slow_period"},
			},
			{
				ID: "donchian_15m", Label: "Donchian Breakout (research)", Interval: "15min",
				LiveEligible: false, LiveBlockNote: "not implemented in Go runner",
				ParamHints: []string{"lookback", "atr_stop"},
			},
		},
	}
}

func labAppRoot() string {
	if r := strings.TrimSpace(os.Getenv("LAB_APP_ROOT")); r != "" {
		return r
	}
	if _, err := os.Stat("scripts/backtest/strategy-matrix.py"); err == nil {
		wd, _ := os.Getwd()
		return wd
	}
	return "/app"
}

func runPythonLab(ctx context.Context, script string, args ...string) ([]byte, error) {
	root := labAppRoot()
	py := strings.TrimSpace(os.Getenv("PYTHON_BIN"))
	if py == "" {
		py = "python3"
	}
	scriptPath := filepath.Join(root, "scripts", "backtest", script)
	if _, err := os.Stat(scriptPath); err != nil {
		return nil, fmt.Errorf("strategy lab script missing at %s (install python3 + scripts on runner)", scriptPath)
	}
	full := append([]string{scriptPath}, args...)
	cmd := exec.CommandContext(ctx, py, full...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PYTHONPATH="+filepath.Join(root, "scripts"),
		"GATEWAY_URL="+gatewayBaseURL(),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New(msg)
	}
	return out, nil
}

func (r *Runner) StrategyLabCompare(ctx context.Context, req StrategyLabCompareRequest) (json.RawMessage, error) {
	ticker := strings.TrimSpace(req.Ticker)
	if ticker == "" {
		return nil, fmt.Errorf("ticker is required")
	}
	days := req.Days
	if days <= 0 {
		days = 90
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	out, err := runPythonLab(ctx, "strategy-matrix.py",
		"--gateway-url", gatewayBaseURL(),
		"--tickers", ticker,
		"--days", fmt.Sprintf("%d", days),
	)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(out), nil
}

func (r *Runner) StrategyLabOptimize(ctx context.Context, req StrategyLabOptimizeRequest) (json.RawMessage, error) {
	uid := strings.TrimSpace(req.UID)
	if uid == "" {
		return nil, fmt.Errorf("uid is required")
	}
	st := strings.TrimSpace(req.Strategy)
	days := req.Days
	if days <= 0 {
		days = 90
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	switch st {
	case "sma_crossover", "sma":
		return runPythonLabAIScanner(ctx, uid, "sma", days, true, false)
	case "level_bounce":
		return runPythonLabAIScanner(ctx, uid, "level_bounce", days, true, false)
	default:
		return r.StrategyLabCompare(ctx, StrategyLabCompareRequest{
			Ticker: req.Ticker, Days: days,
		})
	}
}

func runPythonLabAIScanner(ctx context.Context, uid, strategy string, days int, optimize, trailing bool) (json.RawMessage, error) {
	root := labAppRoot()
	py := "python3"
	if p := os.Getenv("PYTHON_BIN"); p != "" {
		py = p
	}
	scriptPath := filepath.Join(root, "scripts", "ai-scanner", "backtest.py")
	args := []string{
		scriptPath,
		"--gateway-url", gatewayBaseURL(),
		"--uid", uid,
		"--strategy", strategy,
		"--days", fmt.Sprintf("%d", days),
		"--json",
	}
	if optimize {
		args = append(args, "--optimize")
	}
	if trailing {
		args = append(args, "--optimize-trailing")
	}
	cmd := exec.CommandContext(ctx, py, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PYTHONPATH="+filepath.Join(root, "scripts"),
		"GATEWAY_URL="+gatewayBaseURL(),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.New(strings.TrimSpace(stderr.String()))
	}
	return json.RawMessage(out), nil
}

func (r *Runner) StrategyLabApply(ctx context.Context, req StrategyLabApplyRequest) (*StrategyLabApplyResult, error) {
	typ := strings.TrimSpace(req.Type)
	uid := strings.TrimSpace(req.InstrumentUID)
	if typ == "" || uid == "" {
		return nil, fmt.Errorf("type and instrument_uid are required")
	}
	id := strings.TrimSpace(req.InstanceID)
	if id == "" {
		t := strings.TrimSpace(req.Ticker)
		if t == "" {
			t = "inst"
		}
		id = "lab-" + strings.ToLower(t) + "-" + strings.ReplaceAll(typ, "_", "-")
	}
	acct := strings.TrimSpace(req.AccountID)
	if acct == "" {
		acct = strings.TrimSpace(r.strategiesCfg.ClassicAccountID)
	}
	if acct == "" {
		return nil, fmt.Errorf("account_id is required")
	}
	params := req.Params
	if params == nil {
		params = map[string]string{}
	}
	if _, ok := params["quantity"]; !ok {
		params["quantity"] = "1"
	}
	switch typ {
	case "sma_crossover":
		if _, ok := params["interval"]; !ok {
			params["interval"] = "1h"
		}
		if v := params["trailing_stop_pct"]; v == "" || v == "0" {
			return nil, fmt.Errorf("sma_crossover requires trailing_stop_pct > 0 for live trading")
		}
	case "level_bounce":
		if _, ok := params["interval"]; !ok {
			params["interval"] = "15min"
		}
	case "orb_breakout":
		return nil, fmt.Errorf("orb_breakout cannot be applied to live runner")
	}
	inst := config.StrategyInstanceConfig{
		ID:          id,
		Type:        typ,
		AccountID:   acct,
		Instruments: []string{uid},
		Enabled:     req.Enabled,
		Params:      params,
	}
	if !inst.Enabled {
		inst.Enabled = true
	}
	if err := config.UpsertStrategyInstance(r.configPath, inst); err != nil {
		return nil, err
	}
	added, removed, changed, err := r.ReloadConfig(ctx)
	if err != nil {
		return nil, err
	}
	res := &StrategyLabApplyResult{
		Status:     "ok",
		InstanceID: id,
		Reload:     map[string]int{"added": added, "removed": removed, "changed": changed},
	}
	if req.Start {
		if err := r.StartInstanceByID(ctx, id); err != nil {
			res.Started = false
			res.Status = "applied_reload_start_failed: " + err.Error()
		} else {
			res.Started = true
		}
	}
	return res, nil
}
