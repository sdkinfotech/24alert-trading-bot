package strategy

import (
	"context"
	"os"
	"strings"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"
)

// AITraderCorrelationContext tracks lead instrument (e.g. BRM6) vs traded mini (BMM6).
type AITraderCorrelationContext struct {
	LeadTicker   string              `json:"lead_ticker,omitempty"`
	LeadUID      string              `json:"lead_uid,omitempty"`
	LeadMid      float64             `json:"lead_mid,omitempty"`
	LeadChangeBPS float64            `json:"lead_change_bps,omitempty"`
	SpreadBPS    float64             `json:"spread_bps,omitempty"` // lead vs session mid proxy
	Bars5m       []AITraderCandleBar `json:"bars_5m,omitempty"`
	UpdatedAt    string              `json:"updated_at"`
}

func aiTraderCorrelationTicker(sessionTicker string) string {
	v := strings.TrimSpace(os.Getenv("AI_TRADER_CORRELATION_TICKER"))
	if v != "" {
		return v
	}
	switch strings.ToUpper(strings.TrimSpace(sessionTicker)) {
	case "BMM6", "BMH6", "BMU6":
		return "BRM6"
	default:
		return ""
	}
}

func (r *Runner) resolveCorrelationUID(ctx context.Context, leadTicker string) string {
	if leadTicker == "" {
		return ""
	}
	if err := globalInstrumentCatalog.ensure(ctx); err != nil {
		return ""
	}
	globalInstrumentCatalog.mu.RLock()
	defer globalInstrumentCatalog.mu.RUnlock()
	t := strings.ToUpper(leadTicker)
	for _, ins := range globalInstrumentCatalog.items {
		if strings.EqualFold(ins.Ticker, t) && ins.UID != "" {
			return ins.UID
		}
	}
	return ""
}

func (r *Runner) initAITraderCorrelation(ctx context.Context, s *AITraderSession) {
	if s == nil || s.ctxState == nil || r.mdSvc == nil {
		return
	}
	lead := aiTraderCorrelationTicker(s.Ticker)
	uid := r.resolveCorrelationUID(ctx, lead)
	if uid == "" {
		return
	}
	s.ctxState.corrUID = uid
	s.ctxState.corrTicker = lead
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	candles, err := r.mdSvc.GetCandles(ctx, uid, from, now, pb.CandleInterval_CANDLE_INTERVAL_5_MIN)
	if err != nil {
		r.logger.Warn("ai trader correlation warmup", "ticker", lead, "error", err)
		return
	}
	s.ctxState.mu.Lock()
	for _, c := range candles {
		if len(s.ctxState.corrBars5m) >= 48 {
			s.ctxState.corrBars5m = s.ctxState.corrBars5m[1:]
		}
		s.ctxState.corrBars5m = append(s.ctxState.corrBars5m, marketCandleToBar(c))
	}
	s.ctxState.mu.Unlock()
	if r.candleHub != nil {
		ch, cleanup, err := r.candleHub.Subscribe(ctx, uid, pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_FIVE_MINUTES)
		if err == nil {
			go func() {
				defer cleanup()
				for {
					select {
					case <-ctx.Done():
						return
					case c, ok := <-ch:
						if !ok {
							return
						}
						s.ctxState.mu.Lock()
						bar := strategyCandleToBar(c)
						if n := len(s.ctxState.corrBars5m); n > 0 && s.ctxState.corrBars5m[n-1].Time == bar.Time {
							s.ctxState.corrBars5m[n-1] = bar
						} else {
							s.ctxState.corrBars5m = append(s.ctxState.corrBars5m, bar)
							if len(s.ctxState.corrBars5m) > 48 {
								s.ctxState.corrBars5m = s.ctxState.corrBars5m[len(s.ctxState.corrBars5m)-48:]
							}
						}
						s.ctxState.mu.Unlock()
					}
				}
			}()
		}
	}
}

func (st *aiTraderContextState) correlationSnapshot(sessionMid float64) *AITraderCorrelationContext {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.corrUID == "" {
		return nil
	}
	out := &AITraderCorrelationContext{
		LeadTicker: st.corrTicker,
		LeadUID:    st.corrUID,
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if len(st.corrBars5m) > 0 {
		out.Bars5m = append([]AITraderCandleBar(nil), st.corrBars5m...)
		last := st.corrBars5m[len(st.corrBars5m)-1]
		out.LeadMid = last.Close
		if len(st.corrBars5m) >= 2 {
			prev := st.corrBars5m[len(st.corrBars5m)-2].Close
			if prev > 0 {
				out.LeadChangeBPS = (last.Close - prev) / prev * 10000
			}
		}
	}
	if sessionMid > 0 && out.LeadMid > 0 {
		out.SpreadBPS = (out.LeadMid - sessionMid) / sessionMid * 10000
	}
	return out
}

// chartBars5m on session context state
func (st *aiTraderContextState) appendChartBar5m(bar AITraderCandleBar) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.chartBars5m) > 0 && st.chartBars5m[len(st.chartBars5m)-1].Time == bar.Time {
		st.chartBars5m[len(st.chartBars5m)-1] = bar
		return
	}
	st.chartBars5m = append(st.chartBars5m, bar)
	if len(st.chartBars5m) > aiTraderMaxChartBars5m {
		st.chartBars5m = st.chartBars5m[len(st.chartBars5m)-aiTraderMaxChartBars5m:]
	}
}
