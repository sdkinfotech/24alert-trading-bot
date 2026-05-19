package strategy

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/24alert/trading-bot/internal/marketdata"
)

const (
	aiTraderMaxCollectFeed      = 100
	aiTraderCollectFeedThrottle = 8 * time.Second
)

// AITraderCollectEvent is a live transparency line for the dashboard (not LLM fiction).
type AITraderCollectEvent struct {
	Time    string `json:"time"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// AITraderBufferStats counters what is in memory before the next LLM/advisor send.
type AITraderBufferStats struct {
	BookSamples  int     `json:"book_samples"`
	PrintSamples int     `json:"print_samples"`
	ChartBars    int     `json:"chart_bars"`
	LevelCount   int     `json:"level_count"`
	DailyLevels  int     `json:"daily_levels"`
	HourlyLevels int     `json:"hourly_levels"`
	Mid          float64 `json:"mid,omitempty"`
	LastPrice    float64 `json:"last_price,omitempty"`
}

func (s *AITraderSession) appendCollectFeed(kind, message, detail string) {
	if s == nil {
		return
	}
	now := time.Now().UTC()
	if kind == "book" && !s.lastCollectFeedAt.IsZero() && now.Sub(s.lastCollectFeedAt) < aiTraderCollectFeedThrottle {
		return
	}
	if kind == "book" {
		s.lastCollectFeedAt = now
	}
	ev := AITraderCollectEvent{
		Time:    now.Format(time.RFC3339),
		Kind:    kind,
		Message: message,
		Detail:  detail,
	}
	s.CollectFeed = append([]AITraderCollectEvent{ev}, s.CollectFeed...)
	if len(s.CollectFeed) > aiTraderMaxCollectFeed {
		s.CollectFeed = s.CollectFeed[:aiTraderMaxCollectFeed]
	}
}

func countLevelsByPrefix(levels []AITraderLevel, prefix string) int {
	n := 0
	for _, l := range levels {
		if strings.HasPrefix(l.Source, prefix) {
			n++
		}
	}
	return n
}

func (r *Runner) syncAITraderBufferStatsLocked(s *AITraderSession, f *AITraderFeatures) {
	if s == nil {
		return
	}
	st := AITraderBufferStats{}
	if s.collectBuf != nil {
		st.BookSamples = len(s.collectBuf.bookSnaps)
		st.PrintSamples = len(s.collectBuf.printSnaps)
	}
	if s.ctxState != nil {
		s.ctxState.mu.Lock()
		st.ChartBars = len(s.ctxState.chartBars)
		st.LevelCount = len(s.ctxState.levels)
		st.DailyLevels = countLevelsByPrefix(s.ctxState.levels, "daily_")
		st.HourlyLevels = countLevelsByPrefix(s.ctxState.levels, "hourly_")
		if len(s.ctxState.prints) > 0 {
			st.LastPrice = s.ctxState.prints[len(s.ctxState.prints)-1].Price
		}
		s.ctxState.mu.Unlock()
	}
	if f != nil {
		st.Mid = f.Mid
		if st.LastPrice == 0 {
			st.LastPrice = f.Mid
		}
	}
	s.PhaseProgress.BufferStats = st
}

func mergeLevelLists(parts ...[]AITraderLevel) []AITraderLevel {
	seen := map[string]bool{}
	var out []AITraderLevel
	for _, part := range parts {
		for _, l := range part {
			k := fmt.Sprintf("%.6f-%s-%s", l.Price, l.Kind, l.Source)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, l)
		}
	}
	return out
}

func computeHourlyLevels(hourly []marketdata.Candle, hours int) []AITraderLevel {
	if len(hourly) == 0 || hours <= 0 {
		return nil
	}
	start := 0
	if len(hourly) > hours {
		start = len(hourly) - hours
	}
	highs := make([]dailyLevelCand, 0, hours)
	lows := make([]dailyLevelCand, 0, hours)
	for i := start; i < len(hourly); i++ {
		c := hourly[i]
		label := c.Time.UTC().Format("2006-01-02 15:04")
		highs = append(highs, dailyLevelCand{price: c.High, date: label})
		lows = append(lows, dailyLevelCand{price: c.Low, date: label})
	}
	sort.Slice(highs, func(i, j int) bool { return highs[i].price > highs[j].price })
	sort.Slice(lows, func(i, j int) bool { return lows[i].price < lows[j].price })

	out := make([]AITraderLevel, 0, 6)
	for rank, c := range highs {
		if rank >= 3 {
			break
		}
		out = append(out, AITraderLevel{Price: c.price, Kind: "resistance", Source: "hourly_high " + c.date, Rank: rank + 1})
	}
	for rank, c := range lows {
		if rank >= 3 {
			break
		}
		out = append(out, AITraderLevel{Price: c.price, Kind: "support", Source: "hourly_low " + c.date, Rank: rank + 1})
	}
	return out
}
