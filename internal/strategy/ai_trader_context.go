package strategy

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/internal/marketdata"
)

const (
	aiTraderMaxPrints        = 200
	aiTraderMaxBookDigests   = 15
	aiTraderMaxChartBars     = 40
	aiTraderTapeWindowSec    = 60
	aiTraderDailyLevelDays   = 10
	aiTraderFootprintMinutes = 8
	aiTraderDefaultTickSize  = 0.01
)

// AITraderMarketContext is a rolling multi-source snapshot for LLM and dashboard.
type AITraderMarketContext struct {
	ChartBars    []AITraderCandleBar       `json:"chart_bars,omitempty"`
	Footprint    []AITraderFootprintColumn `json:"footprint,omitempty"`
	DOMBook      *AITraderDOMBook          `json:"dom_book,omitempty"`
	Levels       []AITraderLevel           `json:"levels,omitempty"`
	RecentPrints []AITraderPrint           `json:"recent_prints,omitempty"`
	TapeStats    AITraderTapeStats         `json:"tape_stats"`
	BookTimeline []AITraderBookDigest      `json:"book_timeline,omitempty"`
	SceneNotes   []string                  `json:"scene_notes,omitempty"`
	UpdatedAt    string                    `json:"updated_at"`
}

// AITraderFootprintColumn is one minute of volume-at-price (cluster) data.
type AITraderFootprintColumn struct {
	Time     string                  `json:"time"`
	Label    string                  `json:"label"`
	Cells    []AITraderFootprintCell `json:"cells"`
	TotalVol int64                   `json:"total_vol"`
	Delta    int64                   `json:"delta"`
}

type AITraderFootprintCell struct {
	Price   float64 `json:"price"`
	BuyVol  int64   `json:"buy_vol"`
	SellVol int64   `json:"sell_vol"`
	Total   int64   `json:"total"`
}

// AITraderDOMBook is the full limit-order ladder for scalper DOM view.
type AITraderDOMBook struct {
	ObservedAt string              `json:"observed_at"`
	TickSize   float64             `json:"tick_size"`
	BestBid    float64             `json:"best_bid"`
	BestAsk    float64             `json:"best_ask"`
	Bids       []AITraderBookLevel `json:"bids"`
	Asks       []AITraderBookLevel `json:"asks"`
}

type AITraderCandleBar struct {
	Time   string  `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

type AITraderLevel struct {
	Price  float64 `json:"price"`
	Kind   string  `json:"kind"`
	Source string  `json:"source"`
	Rank   int     `json:"rank"`
}

type AITraderPrint struct {
	Time      string  `json:"time"`
	Direction string  `json:"direction"`
	Price     float64 `json:"price"`
	Quantity  int64   `json:"quantity"`
}

type AITraderTapeStats struct {
	WindowSec  int     `json:"window_sec"`
	TradeCount int     `json:"trade_count"`
	BuyVolume  int64   `json:"buy_volume"`
	SellVolume int64   `json:"sell_volume"`
	LastPrice  float64 `json:"last_price"`
	VWAP       float64 `json:"vwap"`
	DeltaPct   float64 `json:"delta_pct"`
	Aggressor  string  `json:"aggressor,omitempty"`
}

type AITraderBookDigest struct {
	Time      string  `json:"time"`
	Mid       float64 `json:"mid"`
	SpreadBPS float64 `json:"spread_bps"`
	Imbalance float64 `json:"imbalance"`
	BidWall   string  `json:"bid_wall"`
	AskWall   string  `json:"ask_wall"`
}

type footprintCell struct {
	buyVol  int64
	sellVol int64
}

type footprintMinute struct {
	start time.Time
	cells map[float64]*footprintCell
}

type aiTraderContextState struct {
	mu           sync.Mutex
	chartBars    []AITraderCandleBar
	levels       []AITraderLevel
	prints       []AITraderPrint
	footprint    []*footprintMinute
	domBook      *AITraderDOMBook
	tickSize     float64
	bookTimeline []AITraderBookDigest
	sceneNotes   []string
	lastBidWall  string
	lastAskWall  string
}

func newAITraderContextState() *aiTraderContextState {
	return &aiTraderContextState{}
}

func (r *Runner) initAITraderContext(ctx context.Context, s *AITraderSession) {
	if s.ctxState == nil {
		s.ctxState = newAITraderContextState()
	}
	r.warmupAITraderContext(ctx, s)
	go r.runAITraderMarketFeeds(ctx, s)
}

func (r *Runner) warmupAITraderContext(ctx context.Context, s *AITraderSession) {
	if r.mdSvc == nil || s.ctxState == nil {
		return
	}
	now := time.Now().UTC()
	from1m := now.Add(-3 * time.Hour)
	candles, err := r.mdSvc.GetCandles(ctx, s.InstrumentID, from1m, now, pb.CandleInterval_CANDLE_INTERVAL_1_MIN)
	if err != nil {
		r.logger.Warn("ai trader chart warmup", "error", err)
	} else {
		for _, c := range candles {
			s.ctxState.appendChartBar(marketCandleToBar(c))
		}
	}
	fromDay := now.AddDate(0, 0, -aiTraderDailyLevelDays*2)
	daily, err := r.mdSvc.GetCandles(ctx, s.InstrumentID, fromDay, now, pb.CandleInterval_CANDLE_INTERVAL_DAY)
	if err != nil {
		r.logger.Warn("ai trader levels warmup", "error", err)
	} else {
		s.ctxState.setLevels(computeDailyLevels(daily, aiTraderDailyLevelDays))
	}
}

func marketCandleToBar(c marketdata.Candle) AITraderCandleBar {
	return AITraderCandleBar{
		Time:   c.Time.UTC().Format(time.RFC3339),
		Open:   c.Open,
		High:   c.High,
		Low:    c.Low,
		Close:  c.Close,
		Volume: c.Volume,
	}
}

func strategyCandleToBar(c Candle) AITraderCandleBar {
	return AITraderCandleBar{
		Time:   c.Time.UTC().Format(time.RFC3339),
		Open:   c.Open,
		High:   c.High,
		Low:    c.Low,
		Close:  c.Close,
		Volume: c.Volume,
	}
}

type dailyLevelCand struct {
	price float64
	date  string
}

func computeDailyLevels(daily []marketdata.Candle, days int) []AITraderLevel {
	if len(daily) == 0 {
		return nil
	}
	start := 0
	if len(daily) > days {
		start = len(daily) - days
	}
	highs := make([]dailyLevelCand, 0, days)
	lows := make([]dailyLevelCand, 0, days)
	for i := start; i < len(daily); i++ {
		c := daily[i]
		date := c.Time.UTC().Format("2006-01-02")
		highs = append(highs, dailyLevelCand{price: c.High, date: date})
		lows = append(lows, dailyLevelCand{price: c.Low, date: date})
	}
	sort.Slice(highs, func(i, j int) bool { return highs[i].price > highs[j].price })
	sort.Slice(lows, func(i, j int) bool { return lows[i].price < lows[j].price })

	out := make([]AITraderLevel, 0, 6)
	for rank, c := range highs {
		if rank >= 3 {
			break
		}
		out = append(out, AITraderLevel{Price: c.price, Kind: "resistance", Source: "daily_high " + c.date, Rank: rank + 1})
	}
	for rank, c := range lows {
		if rank >= 3 {
			break
		}
		out = append(out, AITraderLevel{Price: c.price, Kind: "support", Source: "daily_low " + c.date, Rank: rank + 1})
	}
	return out
}

func (r *Runner) runAITraderMarketFeeds(ctx context.Context, s *AITraderSession) {
	if r.streamMgr == nil || s.ctxState == nil {
		return
	}
	tradeCh, err := r.streamMgr.SubscribeTrades(ctx, s.InstrumentID)
	if err != nil {
		r.logger.Warn("ai trader trades sub", "error", err)
	} else {
		go func() {
			defer func() { _ = r.streamMgr.Unsubscribe(s.InstrumentID, marketdata.SubTrades) }()
			for {
				select {
				case <-ctx.Done():
					return
				case t, ok := <-tradeCh:
					if !ok || t == nil {
						return
					}
					if uid := t.GetInstrumentUid(); uid != "" && uid != s.InstrumentID {
						continue
					}
					s.ctxState.appendPrint(pbTradeToPrint(t))
				}
			}
		}()
	}

	if r.candleHub != nil {
		candleCh, cleanup, err := r.candleHub.Subscribe(ctx, s.InstrumentID, pb.SubscriptionInterval_SUBSCRIPTION_INTERVAL_ONE_MINUTE)
		if err != nil {
			r.logger.Warn("ai trader candles sub", "error", err)
		} else {
			go func() {
				defer cleanup()
				for {
					select {
					case <-ctx.Done():
						return
					case c, ok := <-candleCh:
						if !ok {
							return
						}
						s.ctxState.appendChartBar(strategyCandleToBar(c))
					}
				}
			}()
		}
	}
}

func pbTradeToPrint(t *pb.Trade) AITraderPrint {
	p := AITraderPrint{
		Direction: strings.TrimPrefix(strings.ToLower(t.GetDirection().String()), "trade_direction_"),
		Price:     quotationToFloat(t.GetPrice()),
		Quantity:  t.GetQuantity(),
		Time:      time.Now().UTC().Format(time.RFC3339),
	}
	if ts := t.GetTime(); ts != nil {
		p.Time = ts.AsTime().UTC().Format(time.RFC3339)
	}
	return p
}

func (st *aiTraderContextState) appendPrint(p AITraderPrint) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.prints = append(st.prints, p)
	if len(st.prints) > aiTraderMaxPrints {
		st.prints = st.prints[len(st.prints)-aiTraderMaxPrints:]
	}
	st.appendFootprintLocked(p)
}

func (st *aiTraderContextState) tickSizeLocked() float64 {
	if st.tickSize > 0 {
		return st.tickSize
	}
	return aiTraderDefaultTickSize
}

func roundPriceTick(price, tick float64) float64 {
	if tick <= 0 {
		tick = aiTraderDefaultTickSize
	}
	return math.Round(price/tick) * tick
}

func (st *aiTraderContextState) appendFootprintLocked(p AITraderPrint) {
	ts, err := time.Parse(time.RFC3339, p.Time)
	if err != nil {
		ts = time.Now().UTC()
	}
	ts = ts.UTC().Truncate(time.Minute)
	tick := st.tickSizeLocked()
	price := roundPriceTick(p.Price, tick)

	var col *footprintMinute
	if len(st.footprint) > 0 && st.footprint[len(st.footprint)-1].start.Equal(ts) {
		col = st.footprint[len(st.footprint)-1]
	} else {
		col = &footprintMinute{start: ts, cells: make(map[float64]*footprintCell)}
		st.footprint = append(st.footprint, col)
		if len(st.footprint) > aiTraderFootprintMinutes {
			st.footprint = st.footprint[len(st.footprint)-aiTraderFootprintMinutes:]
		}
	}
	if col.cells[price] == nil {
		col.cells[price] = &footprintCell{}
	}
	dir := strings.ToLower(p.Direction)
	qty := p.Quantity
	if qty <= 0 {
		qty = 1
	}
	if strings.Contains(dir, "buy") {
		col.cells[price].buyVol += qty
	} else if strings.Contains(dir, "sell") {
		col.cells[price].sellVol += qty
	} else {
		col.cells[price].buyVol += qty / 2
		col.cells[price].sellVol += qty - qty/2
	}
}

func (st *aiTraderContextState) setDOMBook(book *marketdata.Orderbook, ticker string) {
	if book == nil {
		return
	}
	tick := inferTickSize(book, ticker)
	bids := make([]AITraderBookLevel, len(book.Bids))
	for i, row := range book.Bids {
		bids[i] = AITraderBookLevel{Price: row.Price, Quantity: row.Quantity}
	}
	asks := make([]AITraderBookLevel, len(book.Asks))
	for i, row := range book.Asks {
		asks[i] = AITraderBookLevel{Price: row.Price, Quantity: row.Quantity}
	}
	var bestBid, bestAsk float64
	if len(bids) > 0 {
		bestBid = bids[0].Price
	}
	if len(asks) > 0 {
		bestAsk = asks[0].Price
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.tickSize = tick
	st.domBook = &AITraderDOMBook{
		ObservedAt: book.Time.UTC().Format(time.RFC3339),
		TickSize:   tick,
		BestBid:    bestBid,
		BestAsk:    bestAsk,
		Bids:       bids,
		Asks:       asks,
	}
}

func inferTickSize(book *marketdata.Orderbook, ticker string) float64 {
	_ = ticker
	var sample float64
	if len(book.Bids) > 0 {
		sample = book.Bids[0].Price
	} else if len(book.Asks) > 0 {
		sample = book.Asks[0].Price
	}
	switch {
	case sample >= 1000:
		return 1
	case sample >= 100:
		return 0.1
	default:
		return 0.01
	}
}

func buildFootprintColumns(minutes []*footprintMinute) []AITraderFootprintColumn {
	out := make([]AITraderFootprintColumn, 0, len(minutes))
	for _, m := range minutes {
		if m == nil {
			continue
		}
		col := AITraderFootprintColumn{
			Time:  m.start.UTC().Format(time.RFC3339),
			Label: m.start.UTC().Format("15:04"),
		}
		prices := make([]float64, 0, len(m.cells))
		for px := range m.cells {
			prices = append(prices, px)
		}
		sort.Float64s(prices)
		for _, px := range prices {
			c := m.cells[px]
			if c == nil {
				continue
			}
			total := c.buyVol + c.sellVol
			if total <= 0 {
				continue
			}
			col.Cells = append(col.Cells, AITraderFootprintCell{
				Price:   px,
				BuyVol:  c.buyVol,
				SellVol: c.sellVol,
				Total:   total,
			})
			col.TotalVol += total
			col.Delta += c.buyVol - c.sellVol
		}
		out = append(out, col)
	}
	return out
}

func (st *aiTraderContextState) appendChartBar(bar AITraderCandleBar) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.chartBars) > 0 && st.chartBars[len(st.chartBars)-1].Time == bar.Time {
		st.chartBars[len(st.chartBars)-1] = bar
		return
	}
	st.chartBars = append(st.chartBars, bar)
	if len(st.chartBars) > aiTraderMaxChartBars {
		st.chartBars = st.chartBars[len(st.chartBars)-aiTraderMaxChartBars:]
	}
}

func (st *aiTraderContextState) setLevels(levels []AITraderLevel) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.levels = levels
}

func (st *aiTraderContextState) appendBookDigest(f *AITraderFeatures) {
	if f == nil {
		return
	}
	bid := formatWall(f.LargestBidWall)
	ask := formatWall(f.LargestAskWall)
	d := AITraderBookDigest{
		Time:      f.ObservedAt,
		Mid:       f.Mid,
		SpreadBPS: f.SpreadBPS,
		Imbalance: f.Imbalance,
		BidWall:   bid,
		AskWall:   ask,
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.lastBidWall != "" && st.lastBidWall != bid {
		st.addSceneNoteLocked(fmt.Sprintf("bid wall changed: %s -> %s", st.lastBidWall, bid))
	}
	if st.lastAskWall != "" && st.lastAskWall != ask {
		st.addSceneNoteLocked(fmt.Sprintf("ask wall changed: %s -> %s", st.lastAskWall, ask))
	}
	st.lastBidWall = bid
	st.lastAskWall = ask
	st.bookTimeline = append(st.bookTimeline, d)
	if len(st.bookTimeline) > aiTraderMaxBookDigests {
		st.bookTimeline = st.bookTimeline[len(st.bookTimeline)-aiTraderMaxBookDigests:]
	}
}

func formatWall(w AITraderWall) string {
	if w.Quantity <= 0 {
		return "none"
	}
	return fmt.Sprintf("%.4f x %d", w.Price, w.Quantity)
}

func (st *aiTraderContextState) addSceneNoteLocked(note string) {
	note = strings.TrimSpace(note)
	if note == "" {
		return
	}
	st.sceneNotes = append(st.sceneNotes, note)
	if len(st.sceneNotes) > 12 {
		st.sceneNotes = st.sceneNotes[len(st.sceneNotes)-12:]
	}
}

func (st *aiTraderContextState) snapshot() AITraderMarketContext {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := AITraderMarketContext{
		TapeStats: computeTapeStatsLocked(st.prints, aiTraderTapeWindowSec),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if len(st.chartBars) > 0 {
		out.ChartBars = append([]AITraderCandleBar(nil), st.chartBars...)
	}
	if len(st.levels) > 0 {
		out.Levels = append([]AITraderLevel(nil), st.levels...)
	}
	if len(st.prints) > 0 {
		out.RecentPrints = append([]AITraderPrint(nil), st.prints...)
	}
	if len(st.bookTimeline) > 0 {
		out.BookTimeline = append([]AITraderBookDigest(nil), st.bookTimeline...)
	}
	if len(st.sceneNotes) > 0 {
		out.SceneNotes = append([]string(nil), st.sceneNotes...)
	}
	if len(st.footprint) > 0 {
		out.Footprint = buildFootprintColumns(st.footprint)
	}
	if st.domBook != nil {
		cp := *st.domBook
		cp.Bids = append([]AITraderBookLevel(nil), st.domBook.Bids...)
		cp.Asks = append([]AITraderBookLevel(nil), st.domBook.Asks...)
		out.DOMBook = &cp
	}
	return out
}

func computeTapeStatsLocked(prints []AITraderPrint, windowSec int) AITraderTapeStats {
	stats := AITraderTapeStats{WindowSec: windowSec}
	if len(prints) == 0 {
		return stats
	}
	cutoff := time.Now().UTC().Add(-time.Duration(windowSec) * time.Second)
	var sumPxVol float64
	var totalVol int64
	for i := len(prints) - 1; i >= 0; i-- {
		p := prints[i]
		ts, err := time.Parse(time.RFC3339, p.Time)
		if err != nil {
			continue
		}
		if ts.Before(cutoff) {
			break
		}
		stats.TradeCount++
		stats.LastPrice = p.Price
		vol := p.Quantity
		if vol <= 0 {
			vol = 1
		}
		totalVol += vol
		sumPxVol += p.Price * float64(vol)
		dir := strings.ToLower(p.Direction)
		if strings.Contains(dir, "buy") {
			stats.BuyVolume += vol
		} else if strings.Contains(dir, "sell") {
			stats.SellVolume += vol
		}
	}
	if totalVol > 0 {
		stats.VWAP = sumPxVol / float64(totalVol)
	}
	den := stats.BuyVolume + stats.SellVolume
	if den > 0 {
		stats.DeltaPct = float64(stats.BuyVolume-stats.SellVolume) / float64(den)
		if stats.DeltaPct > 0.15 {
			stats.Aggressor = "buyers"
		} else if stats.DeltaPct < -0.15 {
			stats.Aggressor = "sellers"
		} else {
			stats.Aggressor = "mixed"
		}
	}
	return stats
}

func (r *Runner) snapshotAITraderContext(s *AITraderSession) *AITraderMarketContext {
	if s == nil || s.ctxState == nil {
		return nil
	}
	snap := s.ctxState.snapshot()
	return &snap
}

func (r *Runner) recordAITraderBookDigest(s *AITraderSession, book *marketdata.Orderbook, f *AITraderFeatures) {
	if s == nil || s.ctxState == nil {
		return
	}
	if book != nil {
		s.ctxState.setDOMBook(book, s.Ticker)
	}
	if f != nil {
		s.ctxState.appendBookDigest(f)
	}
}

func (r *Runner) attachAITraderMarketContext(s *AITraderSession) {
	if s == nil {
		return
	}
	snap := r.snapshotAITraderContext(s)
	s.MarketContext = snap
}
