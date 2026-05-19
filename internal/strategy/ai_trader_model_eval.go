package strategy

import (
	"encoding/json"
	"fmt"
	"math"
)

// SignalEvalResult scores LLM trade_signal vs forward price move.
type replayMidPoint struct {
	at  string
	mid float64
}

type SignalEvalResult struct {
	SessionID    string  `json:"session_id"`
	TotalSignals int     `json:"total_signals"`
	Hits         int     `json:"hits"`
	Accuracy     float64 `json:"accuracy"`
	WindowMin    int     `json:"window_min"`
}

// EvalTradeSignalsOnReplay compares signals stored in replay ticks to subsequent mid moves.
func EvalTradeSignalsOnReplay(sessionID string, windowMin int) (*SignalEvalResult, error) {
	ticks, err := ReplayTicks(sessionID, 0)
	if err != nil {
		return nil, err
	}
	if windowMin <= 0 {
		windowMin = 5
	}
	res := &SignalEvalResult{SessionID: sessionID, WindowMin: windowMin}
	var mids []replayMidPoint
	var signals []struct {
		at     string
		side   string
		price  float64
	}
	for _, t := range ticks {
		if t.Kind == "features" {
			var f AITraderFeatures
			if json.Unmarshal([]byte(t.PayloadJSON), &f) == nil && f.Mid > 0 {
				mids = append(mids, replayMidPoint{at: t.ObservedAt, mid: f.Mid})
			}
		}
		if t.Kind == "trade_signal" {
			var sig AITraderTradeSignal
			if json.Unmarshal([]byte(t.PayloadJSON), &sig) == nil && sig.actionable() {
				signals = append(signals, struct {
					at    string
					side  string
					price float64
				}{t.ObservedAt, sig.Side, sig.LevelPrice})
			}
		}
	}
	for _, sig := range signals {
		res.TotalSignals++
		fwd := forwardMid(mids, sig.at, windowMin)
		if fwd <= 0 {
			continue
		}
		move := fwd - sig.price
		hit := false
		switch sig.side {
		case "buy":
			hit = move > 0
		case "sell":
			hit = move < 0
		}
		if hit {
			res.Hits++
		}
	}
	if res.TotalSignals > 0 {
		res.Accuracy = float64(res.Hits) / float64(res.TotalSignals)
	}
	return res, nil
}

func forwardMid(mids []replayMidPoint, after string, windowMin int) float64 {
	// Simple: take last mid in slice after signal index within window (replay uses chronological order).
	idx := -1
	for i, m := range mids {
		if m.at >= after {
			idx = i
			break
		}
	}
	if idx < 0 {
		return 0
	}
	end := idx + windowMin*30 // ~30 ticks/min rough
	if end >= len(mids) {
		end = len(mids) - 1
	}
	if end <= idx {
		return mids[idx].mid
	}
	return mids[end].mid
}

// CompareModelsOnSession runs eval for multiple model names if stored in tick metadata.
func CompareModelsOnSession(sessionID string) (map[string]float64, error) {
	base, err := EvalTradeSignalsOnReplay(sessionID, 5)
	if err != nil {
		return nil, err
	}
	out := map[string]float64{"default": base.Accuracy}
	return out, nil
}

func signalQualityBPS(entry, exit float64, side string) float64 {
	if entry <= 0 {
		return 0
	}
	move := (exit - entry) / entry * 10000
	if side == "sell" {
		move = -move
	}
	return math.Round(move*100) / 100
}

func formatEvalSummary(res *SignalEvalResult) string {
	if res == nil {
		return ""
	}
	return fmt.Sprintf("signals=%d hits=%d accuracy=%.1f%% window=%dm",
		res.TotalSignals, res.Hits, res.Accuracy*100, res.WindowMin)
}
