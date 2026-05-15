package ledger

import (
	"math"
	"strings"
	"sync"
)

// InstanceLedger tracks position (in instrument units / shares) and average entry price from fills.
// Direction strings match order.OrderRecord.Direction (e.g. ORDER_DIRECTION_BUY from SDK).
type InstanceLedger struct {
	mu sync.RWMutex

	// Quantity in shares (positive = long, negative = short).
	QtyByInst map[string]float64
	// Volume-weighted average entry price per share for the net position.
	AvgByInst map[string]float64
	// Cumulative realized P&L in RUB from closed portions (best-effort, share-priced).
	RealizedRUB float64
}

func NewInstanceLedger() *InstanceLedger {
	return &InstanceLedger{
		QtyByInst: make(map[string]float64),
		AvgByInst: make(map[string]float64),
	}
}

func isBuy(dir string) bool {
	d := strings.ToUpper(strings.TrimSpace(dir))
	return strings.Contains(d, "BUY") && !strings.Contains(d, "UNSPECIFIED")
}

func isSell(dir string) bool {
	d := strings.ToUpper(strings.TrimSpace(dir))
	return strings.Contains(d, "SELL") && !strings.Contains(d, "UNSPECIFIED")
}

// ApplyFill applies executed shares at pricePerShare (RUB) updating position and realized PnL.
// Returns incremental realized P&L in RUB contributed by this fill.
func (l *InstanceLedger) ApplyFill(instrumentUID, direction string, shares float64, pricePerShare float64) float64 {
	if shares <= 0 || instrumentUID == "" {
		return 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	startR := l.RealizedRUB
	cur := l.QtyByInst[instrumentUID]
	avg := l.AvgByInst[instrumentUID]

	switch {
	case isBuy(direction):
		rem := shares
		if cur < 0 {
			cover := math.Min(rem, -cur)
			l.RealizedRUB += cover * (avg - pricePerShare)
			cur += cover
			rem -= cover
			if math.Abs(cur) < 1e-9 {
				cur = 0
				avg = 0
			}
		}
		if rem > 0 {
			newQ := cur + rem
			if math.Abs(newQ) < 1e-9 {
				avg = 0
			} else {
				avg = (cur*avg + rem*pricePerShare) / newQ
			}
			cur = newQ
		}
		if math.Abs(cur) < 1e-9 {
			delete(l.QtyByInst, instrumentUID)
			delete(l.AvgByInst, instrumentUID)
			return l.RealizedRUB - startR
		}
		l.QtyByInst[instrumentUID] = cur
		l.AvgByInst[instrumentUID] = avg

	case isSell(direction):
		rem := shares
		if cur > 0 {
			closed := math.Min(rem, cur)
			l.RealizedRUB += closed * (pricePerShare - avg)
			cur -= closed
			rem -= closed
			if math.Abs(cur) < 1e-9 {
				cur = 0
				avg = 0
			}
		}
		if rem > 0 {
			newQ := cur - rem
			if math.Abs(cur) < 1e-9 {
				avg = pricePerShare
			} else {
				avg = (math.Abs(cur)*avg + rem*pricePerShare) / math.Abs(newQ)
			}
			cur = newQ
		}
		if math.Abs(cur) < 1e-9 {
			delete(l.QtyByInst, instrumentUID)
			delete(l.AvgByInst, instrumentUID)
			return l.RealizedRUB - startR
		}
		l.QtyByInst[instrumentUID] = cur
		l.AvgByInst[instrumentUID] = avg
	default:
		return 0
	}

	if math.Abs(l.QtyByInst[instrumentUID]) < 1e-9 {
		delete(l.QtyByInst, instrumentUID)
		delete(l.AvgByInst, instrumentUID)
	}
	return l.RealizedRUB - startR
}

// Snapshot returns a copy of quantities and averages for metrics / API.
func (l *InstanceLedger) Snapshot() (qty map[string]float64, avg map[string]float64, realized float64) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	qty = make(map[string]float64, len(l.QtyByInst))
	avg = make(map[string]float64, len(l.AvgByInst))
	for k, v := range l.QtyByInst {
		qty[k] = v
	}
	for k, v := range l.AvgByInst {
		avg[k] = v
	}
	return qty, avg, l.RealizedRUB
}

// ReconcileFromBroker overwrites internal position for an instrument when drift exceeds tolerance.
func (l *InstanceLedger) ReconcileFromBroker(instrumentUID string, brokerQtyShares, brokerAvgPrice float64, tol float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cur := l.QtyByInst[instrumentUID]
	if math.Abs(cur-brokerQtyShares) <= tol {
		return false
	}
	l.QtyByInst[instrumentUID] = brokerQtyShares
	if math.Abs(brokerQtyShares) < 1e-9 {
		delete(l.AvgByInst, instrumentUID)
		delete(l.QtyByInst, instrumentUID)
	} else {
		l.AvgByInst[instrumentUID] = brokerAvgPrice
	}
	return true
}

// Registry holds per-strategy-instance ledgers.
type Registry struct {
	mu   sync.Mutex
	inst map[string]*InstanceLedger
}

func NewRegistry() *Registry {
	return &Registry{inst: make(map[string]*InstanceLedger)}
}

func (r *Registry) Ledger(instanceID string) *InstanceLedger {
	r.mu.Lock()
	defer r.mu.Unlock()
	if l, ok := r.inst[instanceID]; ok {
		return l
	}
	l := NewInstanceLedger()
	r.inst[instanceID] = l
	return l
}

func (r *Registry) Remove(instanceID string) {
	r.mu.Lock()
	delete(r.inst, instanceID)
	r.mu.Unlock()
}
