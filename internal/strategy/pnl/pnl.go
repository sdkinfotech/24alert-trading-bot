package pnl

import (
	"github.com/24alert/trading-bot/internal/strategy/ledger"
)

// UnrealizedRUB estimates mark-to-market P&L in RUB using last prices per share.
func UnrealizedRUB(l *ledger.InstanceLedger, marks map[string]float64) float64 {
	if l == nil || len(marks) == 0 {
		return 0
	}
	qty, avg, _ := l.Snapshot()
	var u float64
	for uid, q := range qty {
		if q == 0 {
			continue
		}
		px, ok := marks[uid]
		if !ok || px <= 0 {
			continue
		}
		a := avg[uid]
		u += q * (px - a)
	}
	return u
}
