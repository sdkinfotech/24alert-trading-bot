package strategy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
)

// TradingWindowStatus is the MOEX session window for the dashboard control bar.
type TradingWindowStatus struct {
	Active       bool   `json:"active"`
	NextChangeAt string `json:"next_change_at,omitempty"`
	Label        string `json:"label,omitempty"`
	Window       string `json:"window,omitempty"`
	Timezone     string `json:"timezone,omitempty"`
}

// WatchdogLimitsSnapshot exposes configured loss limits for the UI.
type WatchdogLimitsSnapshot struct {
	Enabled            bool    `json:"enabled"`
	MaxDailyLossRub    float64 `json:"max_daily_loss_rub,omitempty"`
	MaxDrawdownPercent float64 `json:"max_drawdown_pct,omitempty"`
}

// InstanceOperationalStatus aggregates operator-facing state for BotControlBar.
type InstanceOperationalStatus struct {
	InstanceID          string                 `json:"instance_id"`
	Running             bool                   `json:"running"`
	EnabledInConfig     bool                   `json:"enabled_in_config"`
	Type                string                 `json:"type,omitempty"`
	Tickers             string                 `json:"tickers,omitempty"`
	TradingWindow       TradingWindowStatus    `json:"trading_window"`
	Timeframe           string                 `json:"timeframe,omitempty"`
	LastClosedCandleAt  string                 `json:"last_closed_candle_at,omitempty"`
	NextCandleCloseAt   string                 `json:"next_candle_close_at,omitempty"`
	IndicatorAvailable  bool                   `json:"indicator_available"`
	FeedStaleHint       bool                   `json:"feed_stale_hint"`
	BrokerPositionQty   float64                `json:"broker_position_qty"`
	LedgerMismatch      bool                   `json:"ledger_mismatch"`
	ProtectiveStops     int                    `json:"protective_stops"`
	OpenPosition        bool                   `json:"open_position"`
	DailyPnlRub         float64                `json:"daily_pnl_rub"`
	WatchdogLimits      WatchdogLimitsSnapshot `json:"watchdog_limits"`
}

// FlattenResult is returned by POST /instances/{id}/flatten.
type FlattenResult struct {
	Status           string `json:"status"`
	OrdersSubmitted  int    `json:"orders_submitted"`
	InstanceID       string `json:"instance_id"`
}

var (
	errFlattenNotRunning = errors.New("instance is not running")
	errFlattenNoPosition = errors.New("no open broker position for instance instruments")
)

// FlattenInstance closes broker positions for the instance instruments (manual operator action).
func (r *Runner) FlattenInstance(ctx context.Context, instanceID string) (FlattenResult, error) {
	if !r.InstanceRunning(instanceID) {
		return FlattenResult{}, errFlattenNotRunning
	}
	st, err := r.instanceOperationalStatus(ctx, instanceID)
	if err != nil {
		return FlattenResult{}, err
	}
	if !st.OpenPosition {
		return FlattenResult{}, errFlattenNoPosition
	}
	n := r.flattenInstance(ctx, instanceID, "manual_ui")
	if n == 0 {
		return FlattenResult{}, errFlattenNoPosition
	}
	return FlattenResult{
		Status:          "ok",
		OrdersSubmitted: n,
		InstanceID:      instanceID,
	}, nil
}

// InstanceOperationalStatus returns aggregated status for the dashboard control bar.
func (r *Runner) InstanceOperationalStatus(ctx context.Context, instanceID string) (InstanceOperationalStatus, error) {
	return r.instanceOperationalStatus(ctx, instanceID)
}

func (r *Runner) instanceOperationalStatus(ctx context.Context, instanceID string) (InstanceOperationalStatus, error) {
	inst, ok := r.byID[instanceID]
	if !ok {
		return InstanceOperationalStatus{}, fmt.Errorf("unknown instance %q", instanceID)
	}
	now := time.Now()
	out := InstanceOperationalStatus{
		InstanceID:      instanceID,
		Running:         r.InstanceRunning(instanceID),
		EnabledInConfig: inst.Enabled,
		Type:            inst.Type,
		Tickers:         r.InstanceTickers(inst),
		Timeframe:       strings.TrimSpace(inst.Params["interval"]),
	}
	if out.Timeframe == "" {
		out.Timeframe = "5m"
	}
	if r.schedule != nil {
		next, active, label := r.schedule.NextScheduleChange(now)
		out.TradingWindow = TradingWindowStatus{
			Active:       active,
			NextChangeAt: next.UTC().Format(time.RFC3339),
			Label:        label,
			Window:       r.schedule.WindowString(),
			Timezone:     r.schedule.TimezoneName(),
		}
	}
	wd := r.strategiesCfg.Watchdog
	out.WatchdogLimits = WatchdogLimitsSnapshot{
		Enabled:            wd.Enabled,
		MaxDailyLossRub:    wd.MaxDailyLossRub,
		MaxDrawdownPercent: wd.MaxDrawdownPercent,
	}

	if out.Running {
		if _, _, total, _, ok := r.InstancePNLBrokerAware(ctx, instanceID); ok {
			out.DailyPnlRub = total
		}
		if stops, _, err := r.InstanceStopOrders(ctx, instanceID); err == nil {
			out.ProtectiveStops = len(stops)
		}
		var brokerByUID map[string]float64
		if pf, _, err := r.InstancePortfolio(ctx, instanceID); err == nil {
			var brokerQty float64
			brokerByUID = make(map[string]float64)
			for _, p := range pf.Positions {
				if !p.InInstance {
					continue
				}
				brokerByUID[p.InstrumentUID] = p.Quantity
				brokerQty += math.Abs(p.Quantity)
			}
			out.BrokerPositionQty = brokerQty
			out.OpenPosition = brokerQty > 1e-9
		}
		if qty, _, _, ok := r.InstanceLedgerPositions(instanceID); ok {
			out.LedgerMismatch = ledgerMismatchForInstance(brokerByUID, qty)
		}
		if data, ok := r.InstanceIndicatorData(instanceID); ok {
			out.IndicatorAvailable = true
			if last, interval, ok2 := lastClosedCandleFromIndicator(data, inst.Params["interval"]); ok2 {
				out.LastClosedCandleAt = last.UTC().Format(time.RFC3339)
				out.NextCandleCloseAt = last.Add(interval).UTC().Format(time.RFC3339)
				if time.Since(last) > 2*interval {
					out.FeedStaleHint = true
				}
			}
		}
	}
	return out, nil
}

func ledgerMismatchForInstance(broker map[string]float64, ledger map[string]float64) bool {
	if broker == nil && ledger == nil {
		return false
	}
	seen := make(map[string]struct{})
	for uid := range broker {
		seen[uid] = struct{}{}
	}
	for uid := range ledger {
		seen[uid] = struct{}{}
	}
	for uid := range seen {
		bq := 0.0
		if broker != nil {
			bq = broker[uid]
		}
		lq := 0.0
		if ledger != nil {
			lq = ledger[uid]
		}
		if math.Abs(bq-lq) > 1e-6 {
			return true
		}
	}
	return false
}

func lastClosedCandleFromIndicator(data interface{}, intervalParam string) (last time.Time, step time.Duration, ok bool) {
	sub, err := ParseSubscriptionInterval(intervalParam)
	if err != nil {
		sub = pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIVE_MINUTES
	}
	step = IntervalDuration(sub)

	raw, err := json.Marshal(data)
	if err != nil {
		return time.Time{}, step, false
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		return time.Time{}, step, false
	}
	if candlesRaw, exists := env["candles"]; exists {
		var candles []map[string]interface{}
		if err := json.Unmarshal(candlesRaw, &candles); err == nil && len(candles) > 0 {
			if t, ok := parseChartCandleTime(candles[len(candles)-1]["time"]); ok {
				return t, step, true
			}
		}
	}
	var snap struct {
		Candles []struct {
			Time time.Time `json:"time"`
		} `json:"candles"`
	}
	if err := json.Unmarshal(raw, &snap); err == nil && len(snap.Candles) > 0 {
		return snap.Candles[len(snap.Candles)-1].Time, step, true
	}
	return time.Time{}, step, false
}

func flattenEventType(reason string) string {
	if strings.Contains(reason, "manual") {
		return "manual_flatten"
	}
	return "watchdog_flatten"
}
