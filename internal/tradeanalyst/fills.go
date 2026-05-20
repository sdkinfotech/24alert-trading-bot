package tradeanalyst

import "strings"

// Fill is a normalized execution for round-trip extraction.
type Fill struct {
	Time     string
	Side     string
	Price    float64
	Quantity int64
	Note     string
}

// ExtractRounds pairs fills into closed trades (FIFO per side).
func ExtractRounds(sessionID, ticker, uid, account string, fills []Fill, sl, tp, slMult, tpMult float64) []TradeRound {
	if len(fills) == 0 {
		return nil
	}
	var rounds []TradeRound
	var open *TradeRound
	roundIdx := 0

	closeRound := func(ex Fill, exitReason string) {
		if open == nil {
			return
		}
		open.ExitTime = ex.Time
		open.ExitPrice = ex.Price
		open.ExitReason = exitReason
		open.HoldMinutes = minutesBetween(open.EntryTime, open.ExitTime)
		open.PlannedSL = sl
		open.PlannedTP = tp
		open.SLMultATR = slMult
		open.TPMultATR = tpMult
		if open.Side == "long" {
			open.MoveATR = ex.Price - open.EntryPrice
			if ex.Price > open.EntryPrice {
				open.Outcome = "win"
			} else if ex.Price < open.EntryPrice {
				open.Outcome = "loss"
			} else {
				open.Outcome = "flat"
			}
		} else {
			open.MoveATR = open.EntryPrice - ex.Price
			if ex.Price < open.EntryPrice {
				open.Outcome = "win"
			} else if ex.Price > open.EntryPrice {
				open.Outcome = "loss"
			} else {
				open.Outcome = "flat"
			}
		}
		if strings.Contains(strings.ToLower(ex.Note), "stop") {
			open.Tags = append(open.Tags, "sl_hit")
		}
		if strings.Contains(strings.ToLower(ex.Note), "take_profit") || strings.Contains(strings.ToLower(ex.Note), "profit") {
			open.Tags = append(open.Tags, "tp_hit")
		}
		rounds = append(rounds, *open)
		open = nil
	}

	for _, f := range fills {
		qty := f.Quantity
		if f.Side == "sell" {
			qty = -qty
		}
		if open == nil {
			if qty == 0 {
				continue
			}
			side := "long"
			if qty < 0 {
				side = "short"
			}
			roundIdx++
			open = &TradeRound{
				ID: sessionID + "-r" + itoa(roundIdx),
				SessionID: sessionID, Ticker: ticker, InstrumentUID: uid, AccountID: account,
				Side: side, EntryTime: f.Time, EntryPrice: f.Price, Quantity: abs64(qty),
				EntrySource: parseEntrySource(f.Note),
			}
			continue
		}
		// closing or flipping
		if (open.Side == "long" && qty < 0) || (open.Side == "short" && qty > 0) {
			reason := parseExitReason(f.Note)
			closeRound(f, reason)
			if abs64(qty) > 0 {
				side := "long"
				if qty < 0 {
					side = "short"
				}
				roundIdx++
				open = &TradeRound{
					ID: sessionID + "-r" + itoa(roundIdx),
					SessionID: sessionID, Ticker: ticker, InstrumentUID: uid, AccountID: account,
					Side: side, EntryTime: f.Time, EntryPrice: f.Price, Quantity: abs64(qty),
					EntrySource: parseEntrySource(f.Note),
				}
			}
		}
	}
	return rounds
}

func parseEntrySource(note string) string {
	n := strings.ToLower(note)
	switch {
	case strings.Contains(n, "llm"):
		return "llm_signal"
	case strings.Contains(n, "confluence"):
		return "confluence"
	case strings.Contains(n, "broker fill"):
		return "playbook_level"
	default:
		return "unknown"
	}
}

func parseExitReason(note string) string {
	n := strings.ToLower(note)
	switch {
	case strings.Contains(n, "stop_loss"), strings.Contains(n, "stop"):
		return "stop_loss"
	case strings.Contains(n, "take_profit"), strings.Contains(n, "profit"):
		return "take_profit"
	case strings.Contains(n, "flatten"):
		return "flatten"
	case strings.Contains(n, "close"):
		return "close"
	default:
		return "close"
	}
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
