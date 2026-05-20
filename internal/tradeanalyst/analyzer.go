package tradeanalyst

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// SessionInput is a snapshot passed from strategy runner for analysis.
type SessionInput struct {
	SessionID     string
	Ticker        string
	InstrumentUID string
	AccountID     string
	ExecutionMode string
	StrategyKind  string
	StartedAt     string
	StoppedAt     string
	RealizedRUB   float64
	Fills         []Fill
	SLMultATR     float64
	TPMultATR     float64
	LastSL        float64
	LastTP        float64
	ChartBars     []BarInput
	LimitCount    int // working order placements approximated from fills notes
}

// BarInput is one OHLC bar for hourly volatility.
type BarInput struct {
	Time  string
	Open  float64
	High  float64
	Low   float64
	Close float64
}

// AnalyzeSession builds a post-market report from session state + journal.
func AnalyzeSession(in SessionInput, journal []JournalEvent) (*SessionReport, error) {
	if in.SessionID == "" {
		return nil, fmt.Errorf("session_id required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	slMult, tpMult := in.SLMultATR, in.TPMultATR
	if slMult <= 0 {
		slMult = 0.5
	}
	if tpMult <= 0 {
		tpMult = 1.5
	}

	rounds := ExtractRounds(in.SessionID, in.Ticker, in.InstrumentUID, in.AccountID, in.Fills, in.LastSL, in.LastTP, slMult, tpMult)
	atr := sessionATR(in)
	for i := range rounds {
		rounds[i].EntryTiming = judgeEntryTiming(rounds[i], journal, atr)
	}

	links := correlateDecisions(rounds, journal)
	hourly := hourlyVolatility(in.ChartBars, journal)
	freq := reviewFrequency(in, rounds, journal)
	targets := reviewTargets(rounds, slMult, tpMult, atr)
	fit := reviewStrategyFit(in, rounds, journal)

	wins := 0
	for _, tr := range rounds {
		if tr.Outcome == "win" {
			wins++
		}
	}
	winRate := 0.0
	if len(rounds) > 0 {
		winRate = float64(wins) / float64(len(rounds))
	}

	llmN := 0
	for _, ev := range journal {
		if ev.AnalysisSource == "llm" {
			llmN++
		}
	}

	rep := &SessionReport{
		SessionID:     in.SessionID,
		Ticker:        in.Ticker,
		InstrumentUID: in.InstrumentUID,
		AccountID:     in.AccountID,
		ExecutionMode: in.ExecutionMode,
		StartedAt:     in.StartedAt,
		StoppedAt:     in.StoppedAt,
		GeneratedAt:   now,
		TradeRounds:   rounds,
		DecisionLinks: links,
		HourlyVol:     hourly,
		Frequency:     freq,
		Targets:       targets,
		StrategyFit:   fit,
		LLMEvents:     llmN,
		JournalEvents: len(journal),
		RealizedRUB:   in.RealizedRUB,
		WinRate:       winRate,
	}
	rep.Recommendations = buildRecommendations(rep)
	rep.SummaryRU = buildSummaryRU(rep)
	return rep, nil
}

func sessionATR(in SessionInput) float64 {
	if len(in.ChartBars) >= 5 {
		n := len(in.ChartBars)
		if n > 20 {
			n = 20
		}
		bars := in.ChartBars[len(in.ChartBars)-n:]
		sum := 0.0
		for i := 1; i < len(bars); i++ {
			h := bars[i].High
			l := bars[i].Low
			if h > 0 && l > 0 {
				sum += h - l
			}
		}
		if len(bars) > 1 {
			return sum / float64(len(bars)-1)
		}
	}
	return 0.1
}

func correlateDecisions(rounds []TradeRound, journal []JournalEvent) []DecisionLink {
	var links []DecisionLink
	for _, tr := range rounds {
		if ev, off, ok := nearestEvent(journal, tr.EntryTime, 180); ok {
			links = append(links, DecisionLink{
				EventTime: ev.Time, Action: ev.Action, Intent: ev.Intent,
				MarketBias: ev.MarketBias, Summary: trimSummary(ev.Summary, 200),
				OffsetSec: off, LinkKind: "entry",
			})
		}
		if tr.ExitTime != "" {
			if ev, off, ok := nearestEvent(journal, tr.ExitTime, 120); ok {
				links = append(links, DecisionLink{
					EventTime: ev.Time, Action: ev.Action, Intent: ev.Intent,
					MarketBias: ev.MarketBias, Summary: trimSummary(ev.Summary, 200),
					OffsetSec: off, LinkKind: "exit",
				})
			}
		}
	}
	return links
}

func nearestEvent(journal []JournalEvent, at string, maxSec int) (JournalEvent, int, bool) {
	t0, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return JournalEvent{}, 0, false
	}
	best := -1
	var ev JournalEvent
	for _, e := range journal {
		t1, err := time.Parse(time.RFC3339, e.Time)
		if err != nil {
			continue
		}
		d := int(t1.Sub(t0).Seconds())
		if d < 0 {
			d = -d
		}
		if d > maxSec {
			continue
		}
		if best < 0 || d < best {
			best = d
			ev = e
		}
	}
	return ev, best, best >= 0
}

func hourlyVolatility(bars []BarInput, journal []JournalEvent) []HourlyVolStat {
	type acc struct {
		high, low float64
		spread    float64
		nSpread   int
		count     int
	}
	byHour := map[int]*acc{}
	for _, b := range bars {
		t, err := time.Parse(time.RFC3339, b.Time)
		if err != nil {
			continue
		}
		h := t.UTC().Hour()
		a := byHour[h]
		if a == nil {
			a = &acc{high: b.High, low: b.Low}
			byHour[h] = a
		}
		if b.High > a.high {
			a.high = b.High
		}
		if b.Low < a.low || a.low == 0 {
			a.low = b.Low
		}
		a.count++
	}
	for _, ev := range journal {
		if ev.Mid <= 0 {
			continue
		}
		t, err := time.Parse(time.RFC3339, ev.Time)
		if err != nil {
			continue
		}
		h := t.UTC().Hour()
		a := byHour[h]
		if a == nil {
			a = &acc{high: ev.Mid, low: ev.Mid}
			byHour[h] = a
		}
		a.nSpread++
	}
	var hours []int
	for h := range byHour {
		hours = append(hours, h)
	}
	sort.Ints(hours)
	var out []HourlyVolStat
	for _, h := range hours {
		a := byHour[h]
		mid := (a.high + a.low) / 2
		rng := 0.0
		if mid > 0 {
			rng = (a.high - a.low) / mid * 100
		}
		out = append(out, HourlyVolStat{
			HourUTC: h, BarCount: a.count, RangePct: rng, ATR: a.high - a.low, SampleTicks: a.nSpread,
		})
	}
	return out
}

func reviewFrequency(in SessionInput, rounds []TradeRound, journal []JournalEvent) FrequencyReview {
	hours := sessionHours(in.StartedAt, in.StoppedAt)
	if hours < 0.25 {
		hours = 0.25
	}
	limits := in.LimitCount
	if limits == 0 {
		for _, f := range in.Fills {
			if strings.Contains(strings.ToLower(f.Note), "broker fill") {
				limits++
			}
		}
	}
	perHour := float64(len(rounds)+limits) / hours
	fr := FrequencyReview{
		LimitPlacements: limits,
		RoundsClosed:    len(rounds),
		PerHour:         perHour,
	}
	switch {
	case perHour > 4:
		fr.Verdict = "too_often"
		fr.Note = "слишком много попыток входа/лимиток в час — сузить allow_new_entry или confluence"
	case perHour < 0.3 && len(rounds) == 0:
		fr.Verdict = "too_rare"
		fr.Note = "почти не торговал — возможно завышен confluence_min_score или bias блокирует сторону"
	default:
		fr.Verdict = "ok"
		fr.Note = "частота входов в норме"
	}
	return fr
}

func reviewTargets(rounds []TradeRound, slMult, tpMult, atr float64) TargetReview {
	tr := TargetReview{AvgTPMultATR: tpMult, AvgSLMultATR: slMult}
	if len(rounds) == 0 {
		tr.Verdict = "ok"
		tr.Note = "нет закрытых сделок"
		return tr
	}
	tpHit, slHit := 0, 0
	for _, r := range rounds {
		for _, t := range r.Tags {
			if t == "tp_hit" {
				tpHit++
			}
			if t == "sl_hit" {
				slHit++
			}
		}
		if atr > 0 && r.PlannedTP > 0 && r.EntryPrice > 0 {
			dist := math.Abs(r.PlannedTP-r.EntryPrice) / atr
			if dist > 2.5 && r.Outcome != "win" {
				tr.Verdict = "tp_too_far"
			}
		}
	}
	tr.TPHitRate = float64(tpHit) / float64(len(rounds))
	tr.SLHitRate = float64(slHit) / float64(len(rounds))
	if tr.Verdict == "" {
		switch {
		case tr.TPHitRate < 0.1 && tpMult > 1.2:
			tr.Verdict = "tp_too_far"
			tr.Note = "тейк редко достигается при текущем tp_mult_atr"
		case tr.SLHitRate > 0.6:
			tr.Verdict = "sl_too_tight"
			tr.Note = "частые выбивания по стопу"
		default:
			tr.Verdict = "ok"
			tr.Note = "баланс SL/TP приемлемый"
		}
	}
	return tr
}

func reviewStrategyFit(in SessionInput, rounds []TradeRound, journal []JournalEvent) StrategyFitReview {
	sf := StrategyFitReview{Score: 0.5, Verdict: "partial"}
	if in.StrategyKind != "level_intraday" && in.StrategyKind != "" {
		sf.Weaknesses = append(sf.Weaknesses, "не level_intraday")
	}
	confluenceEntries := 0
	llmEntries := 0
	for _, r := range rounds {
		switch r.EntrySource {
		case "confluence", "playbook_level":
			confluenceEntries++
		case "llm_signal":
			llmEntries++
		}
	}
	if confluenceEntries > 0 {
		sf.Strengths = append(sf.Strengths, "входы у уровней playbook")
		sf.Score += 0.15
	}
	if len(rounds) > 0 {
		sf.Score += 0.1
	}
	mismatch := 0
	for _, ev := range journal {
		if ev.AnalysisSource != "llm" {
			continue
		}
		if strings.Contains(ev.Summary, "коротк") && strings.Contains(ev.Summary, "лонг") {
			mismatch++
		}
	}
	if mismatch > 2 {
		sf.Weaknesses = append(sf.Weaknesses, "LLM путает направление позиции")
		sf.Score -= 0.2
	}
	if sf.Score >= 0.65 {
		sf.Verdict = "aligned"
	} else if sf.Score < 0.4 {
		sf.Verdict = "misaligned"
	}
	if sf.Score > 1 {
		sf.Score = 1
	}
	if sf.Score < 0 {
		sf.Score = 0
	}
	return sf
}

func judgeEntryTiming(tr TradeRound, journal []JournalEvent, atr float64) string {
	ev, _, ok := nearestEvent(journal, tr.EntryTime, 300)
	if !ok || ev.Mid <= 0 || atr <= 0 {
		return "unknown"
	}
	diff := tr.EntryPrice - ev.Mid
	if tr.Side == "long" {
		if diff > atr*0.3 {
			return "late"
		}
		if diff < -atr*0.15 {
			return "early"
		}
	} else {
		if diff < -atr*0.3 {
			return "late"
		}
		if diff > atr*0.15 {
			return "early"
		}
	}
	return "ok"
}

func buildRecommendations(rep *SessionReport) []string {
	var rec []string
	switch rep.Frequency.Verdict {
	case "too_often":
		rec = append(rec, "Снизить частоту: entry_min_confidence +0.05, allow_new_entry=false на 1 сессию")
	case "too_rare":
		rec = append(rec, "Ослабить confluence_min_score на 0.3 или разрешить обе стороны при neutral bias")
	}
	switch rep.Targets.Verdict {
	case "tp_too_far":
		rec = append(rec, "Уменьшить tp_mult_atr на 15–25%")
	case "sl_too_tight":
		rec = append(rec, "Увеличить sl_mult_atr на 10% или подтягивать стоп позже")
	}
	if rep.StrategyFit.Verdict == "misaligned" {
		rec = append(rec, "Сверять LiveState с брокером перед LLM; усилить confluence vs LLM narrative")
	}
	for _, tr := range rep.TradeRounds {
		if tr.EntryTiming == "late" {
			rec = append(rec, "Входы часто запоздалые — prefer limit_at_level, не гнаться за mid")
			break
		}
	}
	if len(rec) == 0 {
		rec = append(rec, "Параметры сессии оставить; продолжить сбор статистики")
	}
	return rec
}

func buildSummaryRU(rep *SessionReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Постмаркет %s (%s): сделок %d, win rate %.0f%%, PnL %.2f ₽. ",
		rep.Ticker, rep.SessionID, len(rep.TradeRounds), rep.WinRate*100, rep.RealizedRUB)
	fmt.Fprintf(&b, "Частота: %s. Цели SL/TP: %s. Соответствие стратегии: %s (%s). ",
		rep.Frequency.Verdict, rep.Targets.Verdict, rep.StrategyFit.Verdict, fmtScore(rep.StrategyFit.Score))
	fmt.Fprintf(&b, "LLM-событий %d, связей с сделками %d.", rep.LLMEvents, len(rep.DecisionLinks))
	return b.String()
}

func HintsFromReport(rep *SessionReport) *TradingHints {
	if rep == nil || rep.Ticker == "" {
		return nil
	}
	h := &TradingHints{Ticker: rep.Ticker, Notes: rep.Recommendations}
	switch rep.Frequency.Verdict {
	case "too_often":
		h.BlockNewEntry = true
		h.EntryMinConfidence = 0.58
	case "too_rare":
		h.EntryMinConfidence = 0.52
	}
	switch rep.Targets.Verdict {
	case "tp_too_far":
		h.TPMultScale = 0.85
	case "sl_too_tight":
		h.SLMultScale = 1.1
	}
	// avoid hour with widest range if no trades won
	if len(rep.HourlyVol) > 0 {
		maxH, maxR := -1, 0.0
		for _, hv := range rep.HourlyVol {
			if hv.RangePct > maxR {
				maxR = hv.RangePct
				maxH = hv.HourUTC
			}
		}
		if maxH >= 0 && maxR > 0.8 && rep.WinRate < 0.5 {
			h.AvoidHoursUTC = []int{maxH}
		}
	}
	return h
}

func sessionHours(start, stop string) float64 {
	t0, e0 := time.Parse(time.RFC3339, start)
	t1, e1 := time.Parse(time.RFC3339, stop)
	if e0 != nil || e1 != nil {
		return 1
	}
	return t1.Sub(t0).Hours()
}

func minutesBetween(a, b string) float64 {
	t0, e0 := time.Parse(time.RFC3339, a)
	t1, e1 := time.Parse(time.RFC3339, b)
	if e0 != nil || e1 != nil {
		return 0
	}
	return t1.Sub(t0).Minutes()
}

func trimSummary(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func trimNote(s string) string {
	return strings.TrimSpace(s)
}

func fmtScore(x float64) string {
	return fmt.Sprintf("%.2f", x)
}
