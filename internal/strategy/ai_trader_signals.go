package strategy

import (
	"fmt"
	"strings"
)

// AITraderTradeSignal is a structured execution hint from LLM or rules.
type AITraderTradeSignal struct {
	Side       string  `json:"side"` // buy | sell | none
	LevelPrice float64 `json:"level_price"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason,omitempty"`
	RiskOverride string `json:"risk_override,omitempty"` // hold | cancel_all | flatten
	ReceivedAt string  `json:"received_at,omitempty"`
}

const aiTraderSignalMinConfidence = 0.55

func (sig *AITraderTradeSignal) actionable() bool {
	if sig == nil {
		return false
	}
	side := normalizeTradeSide(sig.Side)
	if side == "" || side == "none" {
		return false
	}
	if sig.LevelPrice <= 0 {
		return false
	}
	return sig.Confidence >= aiTraderSignalMinConfidence
}

func normalizeTradeSide(s string) string {
	switch s {
	case "buy", "sell", "none", "":
		return s
	default:
		return ""
	}
}

// paperFeedbackForLLM summarizes last fills for the confirmation loop.
func paperFeedbackForLLM(st *PaperTradingState) string {
	if st == nil || len(st.Fills) == 0 {
		return ""
	}
	last := st.Fills[len(st.Fills)-1]
	var b strings.Builder
	fmt.Fprintf(&b, "Последний fill: %s %.4f x%d", last.Side, last.Price, last.Quantity)
	if last.Note != "" {
		b.WriteString(" (" + last.Note + ")")
	}
	if st.PositionLots != 0 {
		fmt.Fprintf(&b, "; позиция %d лот @ %.4f", st.PositionLots, st.AvgPrice)
		if st.StopLoss > 0 {
			fmt.Fprintf(&b, "; SL %.4f", st.StopLoss)
		}
		if st.TakeProfit > 0 {
			fmt.Fprintf(&b, "; TP %.4f", st.TakeProfit)
		}
	}
	total := st.RealizedRUB + st.UnrealizedRUB - st.TotalFeesRUB
	fmt.Fprintf(&b, "; PnL net %.2f RUB (fees %.2f)", total, st.TotalFeesRUB)
	return b.String()
}
