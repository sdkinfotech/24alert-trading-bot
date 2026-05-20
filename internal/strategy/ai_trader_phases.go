package strategy

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	AITraderStrategyLevelIntraday = "level_intraday"

	AITraderPhaseCollecting = "collecting"
	AITraderPhaseAnalyzing  = "analyzing"
	AITraderPhaseReady      = "ready"
	AITraderPhaseTrading    = "trading"
)

// AITraderPhaseProgress is exposed to the dashboard for phase UI.
type AITraderPhaseProgress struct {
	CollectSeconds int                 `json:"collect_seconds"`
	MinCollectSec  int                 `json:"min_collect_sec"`
	ReportsReady   []string            `json:"reports_ready,omitempty"`
	TradingReady   bool                `json:"trading_ready"`
	ReadyReason    string              `json:"ready_reason,omitempty"`
	BufferStats    AITraderBufferStats `json:"buffer_stats,omitempty"`
}

type aiTraderBookSample struct {
	At        time.Time
	Mid       float64
	SpreadBPS float64
	Imbalance float64
	BidWall   string
	AskWall   string
}

type aiTraderPrintSample struct {
	At     time.Time
	Price  float64
	Qty    int64
	Side   string
}

type aiTraderCollectBuffer struct {
	started    time.Time
	bookSnaps  []aiTraderBookSample
	printSnaps []aiTraderPrintSample
}

const (
	aiTraderBufferMaxBook   = 120
	aiTraderBufferMaxPrints = 500
)

func aiTraderCollectMinSec() int {
	v := strings.TrimSpace(os.Getenv("AI_TRADER_COLLECT_MIN_SEC"))
	if v == "" {
		return 60
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 30 {
		return 60
	}
	if n > 300 {
		return 300
	}
	return n
}

func aiTraderTradingMinReportTF() string {
	v := strings.TrimSpace(os.Getenv("AI_TRADER_TRADING_MIN_REPORT_TF"))
	if v == "" {
		return "15m"
	}
	return v
}

func newAITraderCollectBuffer() *aiTraderCollectBuffer {
	return &aiTraderCollectBuffer{started: time.Now().UTC()}
}

func (b *aiTraderCollectBuffer) collectSeconds() int {
	if b == nil {
		return 0
	}
	return int(time.Since(b.started).Seconds())
}

func (b *aiTraderCollectBuffer) appendBook(f *AITraderFeatures) {
	if b == nil || f == nil {
		return
	}
	s := aiTraderBookSample{
		At:        time.Now().UTC(),
		Mid:       f.Mid,
		SpreadBPS: f.SpreadBPS,
		Imbalance: f.Imbalance,
		BidWall:   fmt.Sprintf("%.4f x %d", f.LargestBidWall.Price, f.LargestBidWall.Quantity),
		AskWall:   fmt.Sprintf("%.4f x %d", f.LargestAskWall.Price, f.LargestAskWall.Quantity),
	}
	b.bookSnaps = append(b.bookSnaps, s)
	if len(b.bookSnaps) > aiTraderBufferMaxBook {
		b.bookSnaps = b.bookSnaps[len(b.bookSnaps)-aiTraderBufferMaxBook:]
	}
}

func (b *aiTraderCollectBuffer) appendPrints(prints []AITraderPrint) {
	if b == nil || len(prints) == 0 {
		return
	}
	for _, p := range prints {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(p.Time))
		if err != nil {
			t = time.Now().UTC()
		}
		b.printSnaps = append(b.printSnaps, aiTraderPrintSample{
			At: t, Price: p.Price, Qty: p.Quantity, Side: p.Direction,
		})
	}
	if len(b.printSnaps) > aiTraderBufferMaxPrints {
		b.printSnaps = b.printSnaps[len(b.printSnaps)-aiTraderBufferMaxPrints:]
	}
}

func (b *aiTraderCollectBuffer) digestForLLM() string {
	if b == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Observation window: %d seconds, book samples: %d, prints: %d\n",
		b.collectSeconds(), len(b.bookSnaps), len(b.printSnaps)))
	if len(b.bookSnaps) > 0 {
		first := b.bookSnaps[0]
		last := b.bookSnaps[len(b.bookSnaps)-1]
		sb.WriteString(fmt.Sprintf("Mid move: %.4f -> %.4f, spread bps: %.1f -> %.1f, imbalance: %.2f -> %.2f\n",
			first.Mid, last.Mid, first.SpreadBPS, last.SpreadBPS, first.Imbalance, last.Imbalance))
	}
	n := len(b.printSnaps)
	if n > 0 {
		var buyVol, sellVol int64
		start := b.printSnaps[0].At
		end := b.printSnaps[n-1].At
		for _, p := range b.printSnaps {
			if strings.EqualFold(p.Side, "buy") {
				buyVol += p.Qty
			} else {
				sellVol += p.Qty
			}
		}
		sb.WriteString(fmt.Sprintf("Prints %s–%s: count=%d buy_vol=%d sell_vol=%d delta=%d\n",
			start.Format("15:04:05"), end.Format("15:04:05"), n, buyVol, sellVol, buyVol-sellVol))
	}
	return sb.String()
}

func defaultAITraderPhaseProgress() AITraderPhaseProgress {
	return AITraderPhaseProgress{
		MinCollectSec: aiTraderCollectMinSec(),
	}
}

func (r *Runner) tickAITraderPhase(s *AITraderSession, f *AITraderFeatures) {
	if s == nil || s.Status != "running" {
		return
	}
	r.aiTrader.mu.Lock()
	cur := r.aiTrader.findLocked(s.ID)
	if cur == nil {
		r.aiTrader.mu.Unlock()
		return
	}
	if cur.collectBuf == nil {
		cur.collectBuf = newAITraderCollectBuffer()
	}
	prevPhase := cur.Phase
	prevPrintN := 0
	if cur.collectBuf != nil {
		prevPrintN = len(cur.collectBuf.printSnaps)
	}
	cur.collectBuf.appendBook(f)
	var newPrints []AITraderPrint
	if cur.ctxState != nil {
		newPrints = cur.ctxState.snapshotPrints()
		cur.collectBuf.appendPrints(newPrints)
	}
	sec := cur.collectBuf.collectSeconds()
	cur.PhaseProgress.CollectSeconds = sec
	cur.PhaseProgress.MinCollectSec = aiTraderCollectMinSec()
	r.syncAITraderBufferStatsLocked(cur, f)

	if f != nil {
		cur.appendCollectFeed("book", fmt.Sprintf("Стакан: mid %.4f, spread %.1f bps, imb %.2f",
			f.Mid, f.SpreadBPS, f.Imbalance),
			fmt.Sprintf("bid wall %s | ask wall %s",
				fmt.Sprintf("%.4f x %d", f.LargestBidWall.Price, f.LargestBidWall.Quantity),
				fmt.Sprintf("%.4f x %d", f.LargestAskWall.Price, f.LargestAskWall.Quantity)))
	}
	if added := len(cur.collectBuf.printSnaps) - prevPrintN; added > 0 && len(newPrints) > 0 {
		last := newPrints[len(newPrints)-1]
		cur.appendCollectFeed("print", fmt.Sprintf("Лента: +%d принтов (всего %d в буфере)", added, len(cur.collectBuf.printSnaps)),
			fmt.Sprintf("последний %s %.0f @ %.4f", last.Direction, float64(last.Quantity), last.Price))
	}

	switch cur.Phase {
	case AITraderPhaseCollecting:
		if sec >= cur.PhaseProgress.MinCollectSec {
			cur.Phase = AITraderPhaseAnalyzing
			cur.PhaseProgress.ReadyReason = "минутное окно наблюдения завершено, начинаем анализ"
			r.onPhaseCollectingToAnalyzingLocked(cur, f)
		}
	case AITraderPhaseAnalyzing, AITraderPhaseReady:
		r.refreshAdvisorReportsLocked(cur)
		wasReady := cur.wasTradingReady
		r.evaluateTradingReadinessLocked(cur, f)
		if cur.PhaseProgress.TradingReady && !wasReady {
			r.onTradingReadyLocked(cur)
		}
		cur.wasTradingReady = cur.PhaseProgress.TradingReady
	}
	if prevPhase != cur.Phase && cur.Phase == AITraderPhaseReady && !cur.PhaseProgress.TradingReady {
		cur.appendCollectFeed("phase", "Фаза ready: ожидаем финальный playbook", cur.PhaseProgress.ReadyReason)
	}
	cur.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	r.aiTrader.mu.Unlock()
}

func (r *Runner) refreshAdvisorReportsLocked(s *AITraderSession) {
	reports := fetchAdvisorReportsReady(s.ID)
	r.noteAdvisorReportsLocked(s, reports)
	s.PhaseProgress.ReportsReady = reports
}

func fetchAdvisorReportsReady(sessionID string) []string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("ADVISOR_URL")), "/")
	if base == "" {
		base = "http://24alert-advisor-svc:9030"
	}
	var ready []string
	for _, tf := range []string{"5m", "15m", "30m", "1h"} {
		if advisorHasOKReport(base, sessionID, tf) {
			ready = append(ready, tf)
		}
	}
	return ready
}

func advisorHasOKReport(base, sessionID, tf string) bool {
	url := fmt.Sprintf("%s/advisor/sessions/%s/analyses?tf=%s&limit=1", base, sessionID, tf)
	ctx, cancel := contextWithTimeout(4 * time.Second)
	defer cancel()
	req, err := httpNewRequestGET(ctx, url)
	if err != nil {
		return false
	}
	reports, err := decodeAdvisorReports(req)
	if err != nil || len(reports) == 0 {
		return false
	}
	return strings.EqualFold(reports[0].Status, "ok")
}

// contextWithTimeout and http helpers are in ai_trader_advisor_poll.go
