package strategy

import (
	"context"
	"time"

	"github.com/24alert/trading-bot/internal/tradeanalyst"
)

func (r *Runner) initTradeAnalyst() {
	if r.tradeAnalyst != nil {
		return
	}
	var slogger = tradeanalyst.DefaultLogger()
	if r.logger != nil && r.logger.Logger != nil {
		slogger = r.logger.Logger
	}
	svc, err := tradeanalyst.NewService(slogger)
	if err != nil {
		r.logger.Warn("trade analyst disabled", "error", err)
		return
	}
	r.tradeAnalyst = svc
	tradeAnalystHintsLookup = func(ticker string) (*tradeanalyst.TradingHints, bool) {
		return svc.Store().GetHints(ticker)
	}
}

func (r *Runner) scheduleTradeAnalystPostMarket(s *AITraderSession) {
	if r.tradeAnalyst == nil || s == nil {
		return
	}
	in := sessionToAnalystInput(s)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_, err := r.tradeAnalyst.RunPostMarket(in)
		if err != nil && ctx.Err() == nil {
			r.logger.Warn("trade analyst post-market failed", "session", s.ID, "error", err)
		}
	}()
}

func sessionToAnalystInput(s *AITraderSession) tradeanalyst.SessionInput {
	in := tradeanalyst.SessionInput{
		SessionID:     s.ID,
		Ticker:        s.Ticker,
		InstrumentUID: s.InstrumentID,
		AccountID:     s.AccountID,
		ExecutionMode: s.ExecutionMode,
		StrategyKind:  s.StrategyKind,
		StartedAt:     s.StartedAt,
		StoppedAt:     s.StoppedAt,
	}
	if in.StoppedAt == "" {
		in.StoppedAt = time.Now().UTC().Format(time.RFC3339)
	}
	pol := effectivePolicy(s)
	in.SLMultATR = pol.SLMultATR
	in.TPMultATR = pol.TPMultATR
	var fills []tradeanalyst.Fill
	if s.isArmedLive() && s.LiveState != nil {
		in.RealizedRUB = s.LiveState.RealizedRUB
		in.LastSL = s.LiveState.StopLoss
		in.LastTP = s.LiveState.TakeProfit
		in.LimitCount = len(s.LiveState.WorkingOrders)
		for _, f := range s.LiveState.Fills {
			fills = append(fills, tradeanalyst.Fill{
				Time: f.Time, Side: f.Side, Price: f.Price, Quantity: f.Quantity, Note: f.Note,
			})
		}
	} else if s.PaperState != nil {
		in.RealizedRUB = s.PaperState.RealizedRUB
		in.LastSL = s.PaperState.StopLoss
		in.LastTP = s.PaperState.TakeProfit
		for _, f := range s.PaperState.Fills {
			fills = append(fills, tradeanalyst.Fill{
				Time: f.Time, Side: f.Side, Price: f.Price, Quantity: f.Quantity, Note: f.Note,
			})
		}
	}
	in.Fills = fills
	if s.MarketContext != nil {
		for _, b := range s.MarketContext.ChartBars {
			in.ChartBars = append(in.ChartBars, tradeanalyst.BarInput{
				Time: b.Time, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close,
			})
		}
	}
	return in
}

// RunTradeAnalystPostMarket runs analysis for a session id (loads journal; optional live session).
func (r *Runner) RunTradeAnalystPostMarket(sessionID string) (*tradeanalyst.SessionReport, error) {
	r.initTradeAnalyst()
	if r.tradeAnalyst == nil {
		return nil, errTradeAnalystDisabled
	}
	if s, ok := r.AITraderSession(sessionID); ok {
		return r.tradeAnalyst.RunPostMarket(sessionToAnalystInput(s))
	}
	// journal-only: minimal input from persisted report list not available — require session in memory or journal session id in file
	journal, err := tradeanalyst.LoadJournalEvents(r.tradeAnalyst.Store().JournalPath(), sessionID)
	if err != nil {
		return nil, err
	}
	if len(journal) == 0 {
		return nil, errTradeAnalystNoData
	}
	in := tradeanalyst.SessionInput{
		SessionID: sessionID,
		Ticker:    guessTickerFromSessionID(sessionID),
		StartedAt: journal[0].Time,
		StoppedAt: journal[len(journal)-1].Time,
	}
	return r.tradeAnalyst.RunPostMarket(in)
}

func guessTickerFromSessionID(id string) string {
	// ai-trader-bmm6-20260520-115123
	parts := splitSessionID(id)
	if len(parts) >= 3 && parts[0] == "ai" && parts[1] == "trader" {
		return parts[2]
	}
	return ""
}

func splitSessionID(id string) []string {
	var out []string
	cur := ""
	for _, r := range id {
		if r == '-' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

var (
	errTradeAnalystDisabled = errString("trade analyst not available")
	errTradeAnalystNoData   = errString("no journal data for session")
)

type errString string

func (e errString) Error() string { return string(e) }
