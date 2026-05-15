package strategy

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/24alert/trading-bot/pkg/config"
)

// parseChartCandleTime parses time values produced by encoding/json from the dashboard payload.
func parseChartCandleTime(v interface{}) (time.Time, bool) {
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// mergeRESTCandlesIntoIndicator overlays OHLC from mdSvc.GetCandles onto the strategy indicator JSON
// and appends any bars missing from the in-memory stream (e.g. tail of session, in-progress bar).
func (r *Runner) mergeRESTCandlesIntoIndicator(ctx context.Context, inst config.StrategyInstanceConfig, base interface{}) interface{} {
	if r.mdSvc == nil {
		return base
	}
	var uid string
	for _, u := range inst.Instruments {
		u = strings.TrimSpace(u)
		if u != "" {
			uid = u
			break
		}
	}
	if uid == "" {
		return base
	}
	// Same source as runner.startInstance: params["interval"], empty → 5m (ParseSubscriptionInterval).
	trimmedInterval := strings.TrimSpace(inst.Params["interval"])
	subInt, err := ParseSubscriptionInterval(trimmedInterval)
	if err != nil {
		return base
	}
	candleInt, err := SubscriptionToCandleInterval(subInt)
	if err != nil {
		return base
	}

	raw, err := json.Marshal(base)
	if err != nil {
		return base
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return base
	}
	arr, ok := envelope["candles"].([]interface{})
	if !ok || len(arr) == 0 {
		return base
	}

	byUnix := make(map[int64]map[string]interface{})
	var maxUnix int64
	for _, it := range arr {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		ts, ok := parseChartCandleTime(m["time"])
		if !ok {
			continue
		}
		u := ts.Unix()
		cp := make(map[string]interface{}, len(m))
		for k, v := range m {
			cp[k] = v
		}
		byUnix[u] = cp
		if u > maxUnix {
			maxUnix = u
		}
	}
	if len(byUnix) == 0 {
		return base
	}

	// Slight future skew on `to` so the broker includes the in-progress bar.
	now := time.Now().UTC()
	to := now.Add(2 * time.Minute)
	from := now.Add(-10 * 24 * time.Hour)
	if maxUnix > 0 {
		last := time.Unix(maxUnix, 0).UTC()
		tail := last.Add(-72 * time.Hour)
		if tail.Before(from) {
			from = tail
		}
	}
	// Bound dense intervals (e.g. 1m) so REST responses stay reasonable.
	if d := IntervalDuration(subInt); d > 0 && d < time.Hour {
		minFrom := now.Add(-72 * time.Hour)
		if from.Before(minFrom) {
			from = minFrom
		}
	}

	candles, err := r.mdSvc.GetCandlesSkipCache(ctx, uid, from, to, candleInt)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("indicator chart merge: GetCandles failed", "instance", inst.ID, "uid", uid, "error", err)
		}
		return base
	}
	for _, c := range candles {
		u := c.Time.UTC().Unix()
		if dst, ok := byUnix[u]; ok {
			dst["open"] = c.Open
			dst["high"] = c.High
			dst["low"] = c.Low
			dst["close"] = c.Close
			if c.Volume != 0 {
				dst["volume"] = float64(c.Volume)
			}
			continue
		}
		nm := map[string]interface{}{
			"time":  c.Time.UTC().Format(time.RFC3339Nano),
			"open":  c.Open,
			"high":  c.High,
			"low":   c.Low,
			"close": c.Close,
		}
		if c.Volume != 0 {
			nm["volume"] = float64(c.Volume)
		}
		byUnix[u] = nm
	}

	keys := make([]int64, 0, len(byUnix))
	for k := range byUnix {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]interface{}, 0, len(keys))
	for _, k := range keys {
		out = append(out, byUnix[k])
	}
	envelope["candles"] = out
	// Dashboard: which timeframe REST merge used (must match streaming subscription).
	envelope["chart_instrument_uid"] = uid
	envelope["chart_interval_param"] = trimmedInterval
	envelope["chart_subscription_interval"] = subInt.String()
	envelope["chart_rest_interval"] = candleInt.String()
	return envelope
}

// InstanceIndicatorForChart returns indicator JSON merged with broker REST candles for the dashboard chart.
func (r *Runner) InstanceIndicatorForChart(ctx context.Context, id string) (interface{}, bool) {
	base, ok := r.InstanceIndicatorData(id)
	if !ok {
		return nil, false
	}
	inst, ok := r.byID[id]
	if !ok {
		return base, true
	}
	return r.mergeRESTCandlesIntoIndicator(ctx, inst, base), true
}
