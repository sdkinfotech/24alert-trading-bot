package lb

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/24alert/trading-bot/internal/strategy"
)

// CandlePoint holds OHLC + level info for the indicator chart.
type CandlePoint struct {
	Time       time.Time `json:"time"`
	Open       float64   `json:"open"`
	High       float64   `json:"high"`
	Low        float64   `json:"low"`
	Close      float64   `json:"close"`
	Support    float64   `json:"support"`
	Resistance float64   `json:"resistance"`
}

// SignalPoint records when a signal was generated.
type SignalPoint struct {
	Time      time.Time `json:"time"`
	Direction string    `json:"direction"`
	Reason    string    `json:"reason"`
	RefPrice  float64   `json:"ref_price"`
}

// LevelSource explains which daily candle produced a support/resistance level.
type LevelSource struct {
	Price float64 `json:"price"`
	Date  string  `json:"date"`
	Kind  string  `json:"kind"` // "low" for support, "high" for resistance
	Rank  int     `json:"rank"`
}

// IndicatorSnapshot is returned by IndicatorData() for web dashboard visualization.
type IndicatorSnapshot struct {
	StrategyType      string        `json:"strategy_type"`
	Support           []float64     `json:"support"`
	Resistance        []float64     `json:"resistance"`
	SupportSources    []LevelSource `json:"support_sources"`
	ResistanceSources []LevelSource `json:"resistance_sources"`
	LevelMethod       string        `json:"level_method"`
	LevelDays         int           `json:"level_days"`
	ATR               float64       `json:"atr"`
	Position          int64         `json:"position"`
	CurrentDay        string        `json:"current_day"`
	Candles           []CandlePoint `json:"candles"`
	Signals           []SignalPoint `json:"signals"`
}

const maxHistoryLen = 1000

// Bounce implements an intraday mean-reversion strategy at support/resistance.
//
// Daily candles build S/R levels. On 15-min bars the strategy:
//   - BUY when price bounces off support (low near support, close above it)
//   - SELL when price rejects from resistance (high near resistance, close below it)
//   - Stop-loss: 0.5 * ATR beyond the level
//   - Take-profit: 1.5 * ATR in the trade direction
//   - EOD flatten before cutoff
type Bounce struct {
	atrMult    float64 // how close to level to trigger (fraction of ATR)
	slMult     float64 // stop-loss as multiple of ATR
	tpMult     float64 // take-profit as multiple of ATR
	levelDays  int     // number of daily bars for level detection
	qty        int64
	cutoffHour int
	cutoffMin  int
	tz         *time.Location

	// Computed from daily bars fed during warmup
	support           []float64
	resistance        []float64
	supportSources    []LevelSource
	resistanceSources []LevelSource
	atr               float64

	// Daily candle accumulator
	dailyHighs []float64
	dailyLows  []float64
	dailyTRs   []float64
	dailyDates []string
	prevClose  float64

	// Intraday state: pos is confirmed by fills; pending* tracks orders not yet acknowledged.
	pos          int64
	pendingEntry int64 // +1/-1 entry signal sent, awaiting fill or dispatch failure
	pendingExit  bool  // exit signal sent, awaiting fill or dispatch failure
	entryPrice   float64
	stopLoss     float64
	takeProfit   float64
	currentDay   string
	eodSent      bool
	stopped      bool

	history []CandlePoint
	signals []SignalPoint
}

func New() *Bounce {
	loc, _ := time.LoadLocation("Europe/Moscow")
	if loc == nil {
		loc = time.FixedZone("MSK", 3*3600)
	}
	return &Bounce{
		atrMult:    0.4,
		slMult:     0.5,
		tpMult:     1.5,
		levelDays:  10,
		qty:        1,
		cutoffHour: 18,
		cutoffMin:  30,
		tz:         loc,
	}
}

func RegisterBuiltins(reg *strategy.Registry) {
	reg.Register("level_bounce", func() (strategy.Strategy, error) {
		return New(), nil
	})
}

func (b *Bounce) Info() strategy.StrategyInfo {
	return strategy.StrategyInfo{
		ID:          "level_bounce",
		Name:        "Level Bounce",
		Version:     "1.0.0",
		Description: "Intraday mean-reversion at S/R levels with ATR-based stops",
		Author:      "24alert",
	}
}

func (b *Bounce) Configure(params map[string]string) error {
	b.atrMult = floatFrom(params, "atr_mult", 0.4)
	b.slMult = floatFrom(params, "sl_mult", 0.5)
	b.tpMult = floatFrom(params, "tp_mult", 1.5)
	b.levelDays = intFrom(params, "level_days", 10)
	b.qty = int64From(params, "quantity", 1)
	if b.qty < 1 {
		return fmt.Errorf("quantity must be >= 1")
	}
	b.cutoffHour = intFrom(params, "cutoff_hour", 18)
	b.cutoffMin = intFrom(params, "cutoff_min", 30)

	if tz := params["timezone"]; tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return fmt.Errorf("invalid timezone %q: %w", tz, err)
		}
		b.tz = loc
	}

	b.pos = 0
	b.pendingEntry = 0
	b.pendingExit = false
	b.entryPrice = 0
	b.stopLoss = 0
	b.takeProfit = 0
	b.currentDay = ""
	b.eodSent = false
	return nil
}

// OnDailyCandle feeds a completed daily candle for level building.
// Called by the runner during warmup phase with daily bars.
func (b *Bounce) OnDailyCandle(k strategy.Candle) {
	day := k.Time.In(b.tz).Format("2006-01-02")
	if len(b.dailyDates) > 0 && b.dailyDates[len(b.dailyDates)-1] == day {
		b.dailyHighs[len(b.dailyHighs)-1] = k.High
		b.dailyLows[len(b.dailyLows)-1] = k.Low
		if len(b.dailyTRs) > 0 && b.prevClose > 0 {
			tr := math.Max(k.High-k.Low, math.Max(
				math.Abs(k.High-b.prevClose),
				math.Abs(k.Low-b.prevClose)))
			b.dailyTRs[len(b.dailyTRs)-1] = tr
		}
		b.prevClose = k.Close
		b.recomputeLevels()
		return
	}

	b.dailyDates = append(b.dailyDates, day)
	b.dailyHighs = append(b.dailyHighs, k.High)
	b.dailyLows = append(b.dailyLows, k.Low)
	if b.prevClose > 0 {
		tr := math.Max(k.High-k.Low, math.Max(
			math.Abs(k.High-b.prevClose),
			math.Abs(k.Low-b.prevClose)))
		b.dailyTRs = append(b.dailyTRs, tr)
	}
	b.prevClose = k.Close
	b.recomputeLevels()
}

func (b *Bounce) recomputeLevels() {
	n := b.levelDays
	if n > len(b.dailyHighs) {
		n = len(b.dailyHighs)
	}
	if n < 3 {
		return
	}

	start := len(b.dailyHighs) - n
	highs := make([]levelCandidate, 0, n)
	lows := make([]levelCandidate, 0, n)
	for i := start; i < len(b.dailyHighs); i++ {
		date := ""
		if i < len(b.dailyDates) {
			date = b.dailyDates[i]
		}
		highs = append(highs, levelCandidate{price: b.dailyHighs[i], date: date})
		lows = append(lows, levelCandidate{price: b.dailyLows[i], date: date})
	}

	sortCandidatesDesc(highs)
	sortCandidatesAsc(lows)

	b.resistance, b.resistanceSources = topLevelSources(highs, "high")
	b.support, b.supportSources = topLevelSources(lows, "low")

	// ATR from daily TRs
	atrN := 14
	if atrN > len(b.dailyTRs) {
		atrN = len(b.dailyTRs)
	}
	if atrN > 0 {
		sum := 0.0
		for _, tr := range b.dailyTRs[len(b.dailyTRs)-atrN:] {
			sum += tr
		}
		b.atr = sum / float64(atrN)
	}
}

func (b *Bounce) OnCandle(k strategy.Candle) []strategy.Signal {
	if b.stopped || !k.IsComplete {
		return nil
	}

	t := k.Time.In(b.tz)
	day := t.Format("2006-01-02")

	if day != b.currentDay {
		b.currentDay = day
		b.eodSent = false
	}

	// Record candle for visualization
	sup, res := 0.0, 0.0
	if len(b.support) > 0 {
		sup = b.support[0]
	}
	if len(b.resistance) > 0 {
		res = b.resistance[0]
	}
	b.recordCandle(k, sup, res)

	if len(b.support) == 0 || len(b.resistance) == 0 || b.atr <= 0 {
		return nil
	}

	if b.pendingExit || b.pendingEntry != 0 {
		return nil
	}

	threshold := b.atr * b.atrMult

	// EOD flatten
	cutoff := time.Date(t.Year(), t.Month(), t.Day(), b.cutoffHour, b.cutoffMin, 0, 0, b.tz)
	if !t.Before(cutoff) {
		if b.pos != 0 && !b.eodSent {
			dir := "sell"
			reason := "eod close long"
			if b.pos < 0 {
				dir = "buy"
				reason = "eod close short"
			}
			b.eodSent = true
			b.pendingExit = true
			b.recordSignal(k.Time, dir, reason, k.Close)
			return []strategy.Signal{{
				InstrumentUID: k.InstrumentUID,
				Direction:     dir,
				Quantity:      b.qty,
				OrderType:     "market",
				Reason:        reason,
				CandleTime:    k.Time,
			}}
		}
		return nil
	}

	// Check stop-loss / take-profit
	if b.pos > 0 {
		if k.Low <= b.stopLoss {
			b.recordSignal(k.Time, "sell", "stop loss", b.stopLoss)
			b.pendingExit = true
			return []strategy.Signal{{
				InstrumentUID: k.InstrumentUID,
				Direction:     "sell",
				Quantity:      b.qty,
				OrderType:     "market",
				Reason:        "stop loss",
				CandleTime:    k.Time,
			}}
		}
		if k.High >= b.takeProfit {
			b.recordSignal(k.Time, "sell", "take profit", b.takeProfit)
			b.pendingExit = true
			return []strategy.Signal{{
				InstrumentUID: k.InstrumentUID,
				Direction:     "sell",
				Quantity:      b.qty,
				OrderType:     "market",
				Reason:        "take profit",
				CandleTime:    k.Time,
			}}
		}
		return nil
	}
	if b.pos < 0 {
		if k.High >= b.stopLoss {
			b.recordSignal(k.Time, "buy", "stop loss", b.stopLoss)
			b.pendingExit = true
			return []strategy.Signal{{
				InstrumentUID: k.InstrumentUID,
				Direction:     "buy",
				Quantity:      b.qty,
				OrderType:     "market",
				Reason:        "stop loss",
				CandleTime:    k.Time,
			}}
		}
		if k.Low <= b.takeProfit {
			b.recordSignal(k.Time, "buy", "take profit", b.takeProfit)
			b.pendingExit = true
			return []strategy.Signal{{
				InstrumentUID: k.InstrumentUID,
				Direction:     "buy",
				Quantity:      b.qty,
				OrderType:     "market",
				Reason:        "take profit",
				CandleTime:    k.Time,
			}}
		}
		return nil
	}

	// ── Entry signals (flat position only) ──

	// Bounce off support → BUY
	for _, s := range b.support {
		if k.Low <= s+threshold && k.Close > s {
			b.entryPrice = k.Close
			b.stopLoss = s - b.atr*b.slMult
			b.takeProfit = b.entryPrice + b.atr*b.tpMult
			b.pendingEntry = 1
			reason := fmt.Sprintf("bounce S=%.1f", s)
			b.recordSignal(k.Time, "buy", reason, k.Close)
			return []strategy.Signal{{
				InstrumentUID: k.InstrumentUID,
				Direction:     "buy",
				Quantity:      b.qty,
				OrderType:     "market",
				Reason:        reason,
				CandleTime:    k.Time,
			}}
		}
	}

	// Rejection from resistance → SELL
	for _, r := range b.resistance {
		if k.High >= r-threshold && k.Close < r {
			b.entryPrice = k.Close
			b.stopLoss = r + b.atr*b.slMult
			b.takeProfit = b.entryPrice - b.atr*b.tpMult
			b.pendingEntry = -1
			reason := fmt.Sprintf("reject R=%.1f", r)
			b.recordSignal(k.Time, "sell", reason, k.Close)
			return []strategy.Signal{{
				InstrumentUID: k.InstrumentUID,
				Direction:     "sell",
				Quantity:      b.qty,
				OrderType:     "market",
				Reason:        reason,
				CandleTime:    k.Time,
			}}
		}
	}

	return nil
}

func (b *Bounce) OnOrderbook(_ strategy.Orderbook) []strategy.Signal { return nil }

func (b *Bounce) OnExecution(ev strategy.ExecutionEvent) {
	s := strings.ToLower(ev.Status)
	if s == "filled" || s == "partially_filled" {
		if b.pendingEntry != 0 {
			b.pos = b.pendingEntry
			b.pendingEntry = 0
			return
		}
		if b.pendingExit {
			b.pos = 0
			b.pendingExit = false
			b.entryPrice = 0
			b.stopLoss = 0
			b.takeProfit = 0
			return
		}
		return
	}
	if s == "cancelled" || s == "rejected" {
		if strings.Contains(strings.ToLower(ev.Message), "reject") {
			if b.pendingEntry != 0 {
				b.clearEntryLevels()
				b.pendingEntry = 0
				return
			}
			if b.pendingExit {
				b.pendingExit = false
				return
			}
			b.pos = 0
		}
	}
}

func (b *Bounce) clearEntryLevels() {
	b.entryPrice = 0
	b.stopLoss = 0
	b.takeProfit = 0
}

// OnSignalDispatchFailed implements strategy.SignalDispatchFailureHandler.
func (b *Bounce) OnSignalDispatchFailed(sig strategy.Signal, _ string) {
	if b.pendingExit {
		b.pendingExit = false
		rsn := strings.ToLower(sig.Reason)
		if strings.Contains(rsn, "eod close") {
			b.eodSent = false
		}
		return
	}
	if b.pendingEntry != 0 {
		b.clearEntryLevels()
		b.pendingEntry = 0
	}
}

// ResetTradingStateAfterWarmup implements strategy.PostWarmupCleanup.
func (b *Bounce) ResetTradingStateAfterWarmup() {
	b.pos = 0
	b.pendingEntry = 0
	b.pendingExit = false
	b.clearEntryLevels()
	b.eodSent = false
	b.currentDay = ""
	// Warmup replays historical candles and may generate theoretical signals.
	// They were never dispatched to risk/order services, so do not show them as chart markers.
	b.signals = nil
}

func (b *Bounce) Stop() { b.stopped = true }

func (b *Bounce) recordCandle(k strategy.Candle, sup, res float64) {
	b.history = append(b.history, CandlePoint{
		Time: k.Time, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close,
		Support: sup, Resistance: res,
	})
	if len(b.history) > maxHistoryLen {
		b.history = b.history[len(b.history)-maxHistoryLen:]
	}
}

func (b *Bounce) recordSignal(t time.Time, dir, reason string, price float64) {
	b.signals = append(b.signals, SignalPoint{Time: t, Direction: dir, Reason: reason, RefPrice: price})
	if len(b.signals) > maxHistoryLen {
		b.signals = b.signals[len(b.signals)-maxHistoryLen:]
	}
}

// IndicatorData returns the full indicator snapshot for visualization.
func (b *Bounce) IndicatorData() interface{} {
	candles := make([]CandlePoint, len(b.history))
	copy(candles, b.history)
	sigs := make([]SignalPoint, len(b.signals))
	copy(sigs, b.signals)
	sup := make([]float64, len(b.support))
	copy(sup, b.support)
	res := make([]float64, len(b.resistance))
	copy(res, b.resistance)
	supSrc := make([]LevelSource, len(b.supportSources))
	copy(supSrc, b.supportSources)
	resSrc := make([]LevelSource, len(b.resistanceSources))
	copy(resSrc, b.resistanceSources)
	return IndicatorSnapshot{
		StrategyType:      "level_bounce",
		Support:           sup,
		Resistance:        res,
		SupportSources:    supSrc,
		ResistanceSources: resSrc,
		LevelMethod:       "top-3 daily highs and bottom-3 daily lows over level_days",
		LevelDays:         b.levelDays,
		ATR:               b.atr,
		Position:          b.pos,
		CurrentDay:        b.currentDay,
		Candles:           candles,
		Signals:           sigs,
	}
}

// --- StatefulStrategy ---

type lbState struct {
	ATRMult      float64       `json:"atr_mult"`
	SLMult       float64       `json:"sl_mult"`
	TPMult       float64       `json:"tp_mult"`
	LevelDays    int           `json:"level_days"`
	Qty          int64         `json:"qty"`
	CutoffHour   int           `json:"cutoff_hour"`
	CutoffMin    int           `json:"cutoff_min"`
	Support      []float64     `json:"support"`
	Resistance   []float64     `json:"resistance"`
	DailyDates   []string      `json:"daily_dates"`
	ATR          float64       `json:"atr"`
	DailyHighs   []float64     `json:"daily_highs"`
	DailyLows    []float64     `json:"daily_lows"`
	DailyTRs     []float64     `json:"daily_trs"`
	PrevClose    float64       `json:"prev_close"`
	Pos          int64         `json:"pos"`
	PendingEntry int64         `json:"pending_entry"`
	PendingExit  bool          `json:"pending_exit"`
	EntryPrice   float64       `json:"entry_price"`
	StopLoss     float64       `json:"stop_loss"`
	TakeProfit   float64       `json:"take_profit"`
	CurrentDay   string        `json:"current_day"`
	EodSent      bool          `json:"eod_sent"`
	History      []CandlePoint `json:"history,omitempty"`
	Signals      []SignalPoint `json:"signals,omitempty"`
}

func (b *Bounce) Snapshot() ([]byte, error) {
	return json.Marshal(lbState{
		ATRMult: b.atrMult, SLMult: b.slMult, TPMult: b.tpMult,
		LevelDays: b.levelDays, Qty: b.qty,
		CutoffHour: b.cutoffHour, CutoffMin: b.cutoffMin,
		Support: b.support, Resistance: b.resistance, ATR: b.atr,
		DailyDates: b.dailyDates,
		DailyHighs: b.dailyHighs, DailyLows: b.dailyLows, DailyTRs: b.dailyTRs,
		PrevClose:    b.prevClose,
		Pos:          b.pos,
		PendingEntry: b.pendingEntry,
		PendingExit:  b.pendingExit,
		EntryPrice:   b.entryPrice,
		StopLoss:     b.stopLoss, TakeProfit: b.takeProfit,
		CurrentDay: b.currentDay, EodSent: b.eodSent,
		History: b.history, Signals: b.signals,
	})
}

func (b *Bounce) Restore(blob []byte) error {
	if len(blob) == 0 {
		return nil
	}
	var st lbState
	if err := json.Unmarshal(blob, &st); err != nil {
		return err
	}
	if st.ATRMult > 0 {
		b.atrMult = st.ATRMult
	}
	if st.SLMult > 0 {
		b.slMult = st.SLMult
	}
	if st.TPMult > 0 {
		b.tpMult = st.TPMult
	}
	if st.LevelDays > 0 {
		b.levelDays = st.LevelDays
	}
	if st.Qty > 0 {
		b.qty = st.Qty
	}
	b.cutoffHour = st.CutoffHour
	b.cutoffMin = st.CutoffMin
	b.support = st.Support
	b.resistance = st.Resistance
	b.atr = st.ATR
	if len(st.DailyDates) == len(st.DailyHighs) && len(st.DailyDates) == len(st.DailyLows) {
		b.dailyDates = st.DailyDates
		b.dailyHighs = st.DailyHighs
		b.dailyLows = st.DailyLows
		b.dailyTRs = st.DailyTRs
		b.recomputeLevels()
	} else {
		// Old snapshots did not store daily dates. Rebuild daily buffers from
		// fresh warmup instead of appending duplicate bars after restart.
		b.dailyDates = nil
		b.dailyHighs = nil
		b.dailyLows = nil
		b.dailyTRs = nil
	}
	b.prevClose = st.PrevClose
	b.pos = st.Pos
	b.pendingEntry = st.PendingEntry
	b.pendingExit = st.PendingExit
	b.entryPrice = st.EntryPrice
	b.stopLoss = st.StopLoss
	b.takeProfit = st.TakeProfit
	b.currentDay = st.CurrentDay
	b.eodSent = st.EodSent
	b.history = st.History
	b.signals = st.Signals
	return nil
}

// WarmupCandles returns the number of 15-min candles needed for intraday warmup.
func (b *Bounce) WarmupCandles() int { return 4 }

// ChartCandles returns the number of 15-min candles for dashboard visualization.
func (b *Bounce) ChartCandles() int { return 600 }

// DailyWarmupCandles returns the number of daily bars needed to build S/R levels.
func (b *Bounce) DailyWarmupCandles() int { return b.levelDays + 5 }

type levelCandidate struct {
	price float64
	date  string
}

func topLevelSources(items []levelCandidate, kind string) ([]float64, []LevelSource) {
	n := 3
	if len(items) < n {
		n = len(items)
	}
	levels := make([]float64, 0, n)
	sources := make([]LevelSource, 0, n)
	for i := 0; i < n; i++ {
		levels = append(levels, items[i].price)
		sources = append(sources, LevelSource{
			Price: items[i].price,
			Date:  items[i].date,
			Kind:  kind,
			Rank:  i + 1,
		})
	}
	return levels, sources
}

var (
	_ strategy.StatefulStrategy             = (*Bounce)(nil)
	_ strategy.IndicatorProvider            = (*Bounce)(nil)
	_ strategy.WarmupHint                   = (*Bounce)(nil)
	_ strategy.ChartHint                    = (*Bounce)(nil)
	_ strategy.DailyWarmupHint              = (*Bounce)(nil)
	_ strategy.SignalDispatchFailureHandler = (*Bounce)(nil)
	_ strategy.PostWarmupCleanup            = (*Bounce)(nil)
)

// --- sorting helpers ---

func sortCandidatesDesc(a []levelCandidate) {
	for i := 0; i < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			if a[j].price > a[i].price {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}

func sortCandidatesAsc(a []levelCandidate) {
	for i := 0; i < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			if a[j].price < a[i].price {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}

func intFrom(m map[string]string, key string, def int) int {
	if v, ok := m[key]; ok {
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return i
		}
	}
	return def
}

func int64From(m map[string]string, key string, def int64) int64 {
	if v, ok := m[key]; ok {
		if i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return i
		}
	}
	return def
}

func floatFrom(m map[string]string, key string, def float64) float64 {
	if v, ok := m[key]; ok {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
	}
	return def
}
