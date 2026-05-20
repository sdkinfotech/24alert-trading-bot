package strategy

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/24alert/trading-bot/pkg/metrics"
)

// AITraderMicroSignal is a deterministic tape/book event.
type AITraderMicroSignal struct {
	Kind      string  `json:"kind"` // absorption | sweep | iceberg | spoof
	Side      string  `json:"side"` // buy | sell
	Price     float64 `json:"price"`
	Strength  float64 `json:"strength"` // 0..1
	Detail    string  `json:"detail,omitempty"`
	ObservedAt string `json:"observed_at"`
}

type wallSnapshot struct {
	side     string
	price    float64
	quantity int64
	seenAt   time.Time
}

type aiTraderMicroState struct {
	lastWalls map[string]wallSnapshot
	lastMid   float64
}

func newAITraderMicroState() *aiTraderMicroState {
	return &aiTraderMicroState{lastWalls: make(map[string]wallSnapshot)}
}

func (r *Runner) detectMicrostructure(s *AITraderSession, f *AITraderFeatures, mctx *AITraderMarketContext) []AITraderMicroSignal {
	if s == nil || f == nil || s.ctxState == nil {
		return nil
	}
	st := s.ctxState.micro
	if st == nil {
		st = newAITraderMicroState()
		s.ctxState.micro = st
	}
	var out []AITraderMicroSignal
	now := time.Now().UTC()

	// Wall pull / spoof: wall disappeared without tape at price.
	for _, w := range []AITraderWall{f.LargestBidWall, f.LargestAskWall} {
		if w.Quantity <= 0 {
			continue
		}
		key := w.Side + ":" + formatPriceKey(w.Price)
		prev, ok := st.lastWalls[key]
		if ok && prev.quantity > w.Quantity*2 && prev.quantity > 50 {
			if !printsNearPrice(mctx, w.Price, 3) {
				sig := AITraderMicroSignal{
					Kind: "spoof", Side: w.Side, Price: prev.price, Strength: 0.7,
					Detail: "wall pulled without tape",
					ObservedAt: now.Format(time.RFC3339),
				}
				out = append(out, sig)
				metrics.AITraderMicroSignalsTotal.WithLabelValues("spoof").Inc()
			}
		}
		st.lastWalls[key] = wallSnapshot{side: w.Side, price: w.Price, quantity: w.Quantity, seenAt: now}
	}

	// Absorption: prints at level while wall eaten.
	if mctx != nil {
		for _, p := range mctx.RecentPrints {
			vol := printsVolumeAtPrice(mctx, p.Price, 5)
			if vol < 20 {
				continue
			}
			if p.Direction == "buy" && f.LargestAskWall.Price > 0 && math.Abs(p.Price-f.LargestAskWall.Price) < tickEpsilon(f.Mid) {
				sig := AITraderMicroSignal{
					Kind: "absorption", Side: "buy", Price: p.Price, Strength: 0.8,
					Detail: "buyers absorbing ask wall",
					ObservedAt: now.Format(time.RFC3339),
				}
				out = append(out, sig)
				metrics.AITraderMicroSignalsTotal.WithLabelValues("absorption").Inc()
				break
			}
			if p.Direction == "sell" && f.LargestBidWall.Price > 0 && math.Abs(p.Price-f.LargestBidWall.Price) < tickEpsilon(f.Mid) {
				sig := AITraderMicroSignal{
					Kind: "absorption", Side: "sell", Price: p.Price, Strength: 0.8,
					Detail: "sellers absorbing bid wall",
					ObservedAt: now.Format(time.RFC3339),
				}
				out = append(out, sig)
				metrics.AITraderMicroSignalsTotal.WithLabelValues("absorption").Inc()
				break
			}
		}
	}

	// Sweep: fast move through levels in tape window.
	if mctx != nil && len(mctx.RecentPrints) >= 5 && st.lastMid > 0 {
		first := mctx.RecentPrints[0].Price
		last := mctx.RecentPrints[len(mctx.RecentPrints)-1].Price
		moveBps := math.Abs(last-first) / st.lastMid * 10000
		if moveBps > 8 && len(mctx.RecentPrints) >= 8 {
			side := "buy"
			if last < first {
				side = "sell"
			}
			sig := AITraderMicroSignal{
				Kind: "sweep", Side: side, Price: last, Strength: math.Min(1, moveBps/20),
				Detail: "rapid tape traverse",
				ObservedAt: now.Format(time.RFC3339),
			}
			out = append(out, sig)
			metrics.AITraderMicroSignalsTotal.WithLabelValues("sweep").Inc()
		}
	}
	out = append(out, detectIcebergSignals(mctx, f)...)
	out = append(out, detectMMFlicker(st, f)...)

	st.lastMid = f.Mid

	if len(out) > 10 {
		out = out[len(out)-10:]
	}
	return out
}

// detectIcebergSignals: repeated small prints at same price while visible book qty is low.
func detectIcebergSignals(mctx *AITraderMarketContext, f *AITraderFeatures) []AITraderMicroSignal {
	if mctx == nil || f == nil || len(mctx.RecentPrints) < 4 {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	byPrice := map[float64]struct {
		buy, sell, n int64
	}{}
	eps := tickEpsilon(f.Mid)
	for _, p := range mctx.RecentPrints {
		if math.Abs(p.Price-f.Mid)/f.Mid > 0.002 {
			continue
		}
		b := byPrice[p.Price]
		b.n++
		if strings.Contains(strings.ToLower(p.Direction), "buy") {
			b.buy += p.Quantity
		} else {
			b.sell += p.Quantity
		}
		byPrice[p.Price] = b
	}
	var out []AITraderMicroSignal
	for px, agg := range byPrice {
		if agg.n < 3 {
			continue
		}
		visible := int64(0)
		if math.Abs(px-f.LargestAskWall.Price) <= eps {
			visible = f.LargestAskWall.Quantity
		}
		if math.Abs(px-f.LargestBidWall.Price) <= eps {
			visible = f.LargestBidWall.Quantity
		}
		if visible > 0 && agg.buy+agg.sell > visible*2 {
			side := "sell"
			if agg.buy > agg.sell {
				side = "buy"
			}
			out = append(out, AITraderMicroSignal{
				Kind: "iceberg", Side: side, Price: px, Strength: 0.75,
				Detail: fmt.Sprintf("hidden %s: prints %d vol %d vs visible %d", side, agg.n, agg.buy+agg.sell, visible),
				ObservedAt: now,
			})
			metrics.AITraderMicroSignalsTotal.WithLabelValues("iceberg").Inc()
		}
	}
	return out
}

func detectMMFlicker(st *aiTraderMicroState, f *AITraderFeatures) []AITraderMicroSignal {
	if st == nil || f == nil {
		return nil
	}
	now := time.Now().UTC()
	var out []AITraderMicroSignal
	for _, w := range []AITraderWall{f.LargestBidWall, f.LargestAskWall} {
		if w.Quantity <= 0 {
			continue
		}
		key := w.Side + ":" + formatPriceKey(w.Price)
		prev, ok := st.lastWalls[key]
		if !ok {
			continue
		}
		if prev.quantity > 80 && w.Quantity < prev.quantity/3 && now.Sub(prev.seenAt) < 8*time.Second {
			out = append(out, AITraderMicroSignal{
				Kind: "mm_flicker", Side: w.Side, Price: w.Price, Strength: 0.6,
				Detail: "MM wall flicker / quote pull",
				ObservedAt: now.Format(time.RFC3339),
			})
		}
	}
	return out
}

func formatPriceKey(p float64) string {
	return fmt.Sprintf("%.6f", p)
}

func printsNearPrice(mctx *AITraderMarketContext, price float64, minCount int) bool {
	if mctx == nil {
		return false
	}
	n := 0
	eps := tickEpsilon(price)
	for _, p := range mctx.RecentPrints {
		if math.Abs(p.Price-price) <= eps {
			n++
		}
	}
	return n >= minCount
}

func printsVolumeAtPrice(mctx *AITraderMarketContext, price float64, minCount int) int64 {
	if mctx == nil {
		return 0
	}
	var vol int64
	eps := tickEpsilon(price)
	for _, p := range mctx.RecentPrints {
		if math.Abs(p.Price-price) <= eps {
			vol += p.Quantity
		}
	}
	if vol < int64(minCount) {
		return 0
	}
	return vol
}

func tickEpsilon(mid float64) float64 {
	if mid > 1000 {
		return 0.1
	}
	if mid > 100 {
		return 0.01
	}
	return 0.001
}
