package strategy

import (
	"fmt"
	"time"

	"github.com/24alert/trading-bot/pkg/metrics"
)

// PaperTradingState is virtual execution state for level_intraday sessions.
type PaperTradingState struct {
	PositionLots int64          `json:"position_lots"`
	AvgPrice     float64        `json:"avg_price"`
	RealizedRUB  float64        `json:"realized_rub"`
	WorkingOrders []PaperOrder  `json:"working_orders,omitempty"`
	Fills        []PaperFill    `json:"fills,omitempty"`
	UpdatedAt    string         `json:"updated_at"`
}

type PaperOrder struct {
	ID       string  `json:"id"`
	Side     string  `json:"side"` // buy | sell
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
	LevelRef string  `json:"level_ref,omitempty"`
	Status   string  `json:"status"` // working | filled | cancelled
	PlacedAt string  `json:"placed_at"`
}

type PaperFill struct {
	Time     string  `json:"time"`
	Side     string  `json:"side"`
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
	Note     string  `json:"note,omitempty"`
}

func newPaperTradingState() *PaperTradingState {
	return &PaperTradingState{UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
}

func (r *Runner) startPaperTradingFromPlaybook(s *AITraderSession, f *AITraderFeatures) {
	if s == nil || f == nil || s.LevelPlaybook == nil {
		return
	}
	if s.PaperState == nil {
		s.PaperState = newPaperTradingState()
	}
	mid := f.Mid
	if mid <= 0 {
		return
	}
	levels := s.LevelPlaybook.Levels
	if sup, ok := nearestSupport(levels, mid); ok {
		s.PaperState.WorkingOrders = append(s.PaperState.WorkingOrders, PaperOrder{
			ID: fmt.Sprintf("po-%d", time.Now().UnixNano()), Side: "buy", Price: sup.Price,
			Quantity: 1, LevelRef: sup.Source, Status: "working", PlacedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}
	if res, ok := nearestResistance(levels, mid); ok {
		s.PaperState.WorkingOrders = append(s.PaperState.WorkingOrders, PaperOrder{
			ID: fmt.Sprintf("po-%d", time.Now().UnixNano()+1), Side: "sell", Price: res.Price,
			Quantity: 1, LevelRef: res.Source, Status: "working", PlacedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}
	s.PaperState.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (r *Runner) tickPaperTrading(s *AITraderSession, f *AITraderFeatures) {
	if s == nil || s.PaperState == nil || f == nil || f.Mid <= 0 {
		return
	}
	mid := f.Mid
	remaining := make([]PaperOrder, 0, len(s.PaperState.WorkingOrders))
	for _, o := range s.PaperState.WorkingOrders {
		if o.Status != "working" {
			continue
		}
		filled := false
		switch o.Side {
		case "buy":
			if mid <= o.Price {
				filled = true
			}
		case "sell":
			if mid >= o.Price {
				filled = true
			}
		}
		if !filled {
			remaining = append(remaining, o)
			continue
		}
		fill := PaperFill{
			Time: time.Now().UTC().Format(time.RFC3339), Side: o.Side,
			Price: o.Price, Quantity: o.Quantity, Note: "level touch " + o.LevelRef,
		}
		s.PaperState.Fills = append(s.PaperState.Fills, fill)
		applyPaperFill(s.PaperState, fill)
		metrics.AITraderOrdersTotal.WithLabelValues("paper", o.Side, "filled").Inc()
	}
	s.PaperState.WorkingOrders = remaining
	if len(s.PaperState.WorkingOrders) < s.Limits.MaxActiveOrders {
		// keep at most MaxActiveOrders; no auto-replace in v1
	}
	s.PaperState.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func applyPaperFill(st *PaperTradingState, f PaperFill) {
	if st == nil {
		return
	}
	qty := f.Quantity
	if f.Side == "sell" {
		qty = -qty
	}
	if st.PositionLots == 0 {
		st.PositionLots = qty
		st.AvgPrice = f.Price
		return
	}
	// close or flip simplified
	if (st.PositionLots > 0 && qty < 0) || (st.PositionLots < 0 && qty > 0) {
		closed := min64(abs64(st.PositionLots), abs64(qty))
		pnl := float64(closed) * (f.Price - st.AvgPrice)
		if st.PositionLots < 0 {
			pnl = -pnl
		}
		st.RealizedRUB += pnl
	}
	st.PositionLots += qty
	if st.PositionLots != 0 {
		st.AvgPrice = f.Price
	} else {
		st.AvgPrice = 0
	}
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
